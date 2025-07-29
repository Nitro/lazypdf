#include <jemalloc/jemalloc.h>
#include <pthread.h>
#include <string.h>
#include "main.h"

typedef struct {
  fz_rect bounds;
  fz_matrix ctm;
} dimension;

typedef struct {
  size_t size;
#if defined(_M_IA64) || defined(_M_AMD64)
  size_t align;
#endif
} trace_header;

typedef struct {
  size_t current;
  size_t peak;
  size_t total;
  size_t allocs;
  size_t mem_limit;
  size_t alloc_limit;
} trace_info;

fz_context *global_ctx;
fz_locks_context *global_ctx_lock;
pthread_mutex_t *global_ctx_mutex;
trace_info *tinfo;
fz_alloc_context *trace_alloc_ctx;

static void *trace_malloc(void *arg, size_t size) {
  trace_info *info = (trace_info *) arg;
  trace_header *p;
  if (size == 0)
    return NULL;
  if (size > SIZE_MAX - sizeof(trace_header))
    return NULL;
  p = je_malloc(size + sizeof(trace_header));
  if (p == NULL)
    return NULL;
  p[0].size = size;
  info->current += size;
  info->total += size;
  if (info->current > info->peak)
    info->peak = info->current;
  info->allocs++;
  return (void *)&p[1];
}

static void trace_free(void *arg, void *p_) {
  trace_info *info = (trace_info *) arg;
  trace_header *p = (trace_header *)p_;

  if (p == NULL)
    return;
  info->current -= p[-1].size;
  je_free(&p[-1]);
}

static void *trace_realloc(void *arg, void *p_, size_t size) {
  trace_info *info = (trace_info *) arg;
  trace_header *p = (trace_header *)p_;
  size_t oldsize;

  if (size == 0) {
    trace_free(arg, p_);
    return NULL;
  }
  if (p == NULL)
    return trace_malloc(arg, size);
  if (size > SIZE_MAX - sizeof(trace_header))
    return NULL;
  oldsize = p[-1].size;
  p = je_realloc(&p[-1], size + sizeof(trace_header));
  if (p == NULL)
    return NULL;
  info->current += size - oldsize;
  if (size > oldsize)
    info->total += size - oldsize;
  if (info->current > info->peak)
    info->peak = info->current;
  p[0].size = size;
  info->allocs++;
  return &p[1];
}

void fail(char *msg) {
  fprintf(stderr, "%s\n", msg);
  abort();
}

void lock_mutex(void *user, int lock) {
  pthread_mutex_t *mutex = (pthread_mutex_t *) user;
  if (pthread_mutex_lock(&mutex[lock]) != 0) {
    fail("pthread_mutex_lock()");
  }
}

void unlock_mutex(void *user, int lock) {
  pthread_mutex_t *mutex = (pthread_mutex_t *) user;
  if (pthread_mutex_unlock(&mutex[lock]) != 0) {
    fail("pthread_mutex_unlock()");
  }
}

void init() {
  global_ctx_mutex = je_malloc(sizeof(pthread_mutex_t) * FZ_LOCK_MAX);
  for (size_t i = 0; i < FZ_LOCK_MAX; i++) {
    if (pthread_mutex_init(&global_ctx_mutex[i], NULL) != 0) {
      fail("pthread_mutex_init()");
    }
  }

  global_ctx_lock = je_malloc(sizeof(fz_locks_context));
  global_ctx_lock->user = global_ctx_mutex;
  global_ctx_lock->lock = lock_mutex;
  global_ctx_lock->unlock = unlock_mutex;

  tinfo = je_malloc(sizeof(trace_info));
  tinfo->current = 0;
  tinfo->peak = 0;
  tinfo->total = 0;
  tinfo->allocs = 0;
  tinfo->mem_limit = 0;
  tinfo->alloc_limit = 0;

  trace_alloc_ctx = je_malloc(sizeof(fz_alloc_context));
  trace_alloc_ctx->user = tinfo;
  trace_alloc_ctx->malloc = trace_malloc;
  trace_alloc_ctx->realloc = trace_realloc;
  trace_alloc_ctx->free = trace_free;

  global_ctx = fz_new_context(trace_alloc_ctx, global_ctx_lock, FZ_STORE_DEFAULT);
  fz_register_document_handlers(global_ctx);
  fz_set_error_callback(global_ctx, NULL, NULL);
  fz_set_warning_callback(global_ctx, NULL, NULL);
}

page_count_output page_count(page_count_input input) {
  page_count_output output;
  output.count = 0;
  output.error = NULL;

  fz_context *ctx = fz_clone_context(global_ctx);
  if (ctx == NULL) {
    output.error = strdup("fail to create a context");
    return output;
  }

  fz_stream *stream = NULL;
  pdf_document *doc = NULL;

  fz_var(stream);
  fz_var(doc);

  fz_try(ctx) {
    stream = fz_open_memory(ctx, (const unsigned char *)input.payload, input.payload_length);
    doc = pdf_open_document_with_stream(ctx, stream);
    output.count = pdf_count_pages(ctx, doc);
  } fz_always(ctx) {
    pdf_drop_document(ctx, doc);
    fz_drop_stream(ctx, stream);
  } fz_catch(ctx) {
    output.error = strdup(fz_caught_message(ctx));
  }
  fz_drop_context(ctx);

  return output;
}

static pdf_obj *pdf_lookup_inherited_page_item(fz_context *ctx, pdf_obj *node, pdf_obj *key) {
  pdf_obj *node2 = node;
  pdf_obj *val;

  fz_try(ctx) {
    do {
      val = pdf_dict_get(ctx, node, key);
      if (val)
        break;
      if (pdf_mark_obj(ctx, node))
        fz_throw(ctx, FZ_ERROR_GENERIC, "cycle in page tree (parents)");
      node = pdf_dict_get(ctx, node, PDF_NAME(Parent));
    }
    while (node);
  }
  fz_always(ctx) {
    do {
      pdf_unmark_obj(ctx, node2);
      if (node2 == node)
        break;
      node2 = pdf_dict_get(ctx, node2, PDF_NAME(Parent));
    }
    while (node2);
  }
  fz_catch(ctx) {
    fz_rethrow(ctx);
  }

  return val;
}

int get_rotation(fz_context *ctx, pdf_page *page) {
  pdf_obj *page_obj = page->obj;
  return pdf_to_int(ctx, pdf_lookup_inherited_page_item(ctx, page_obj, PDF_NAME(Rotate)));
}

dimension calculate_dimensions(fz_context *ctx, pdf_page *page, int width, float scale, int dpi) {
  dimension d;
  float scale_factor = 1.5;
  d.bounds = pdf_bound_page(ctx, page, FZ_CROP_BOX);
  if (width != 0) {
    scale_factor = width / d.bounds.x1;
  } else if (scale != 0) {
    scale_factor = scale;
  } else if ((d.bounds.x1 - d.bounds.x0) > (d.bounds.y1 - d.bounds.y0)) {
    switch (get_rotation(ctx, page)) {
      case 0:
      case 180:
        scale_factor = 1;
        break;
      default:
        scale_factor = 1.5;
    }
  }
  float resolution = (float)(dpi) / 72;
  d.ctm = fz_concat(fz_scale(resolution, resolution), fz_scale(scale_factor, scale_factor));
  d.bounds = fz_transform_rect(d.bounds, d.ctm);

  return d;
}

save_to_png_output save_to_png_with_document(fz_context *ctx, pdf_document *doc, save_to_png_params params) {
  save_to_png_output output;
  output.payload = NULL;
  output.payload_length = 0;
  output.error = NULL;

  pdf_page *page = NULL;
  fz_device *device = NULL;
  fz_pixmap *pixmap = NULL;
  fz_buffer *buffer = NULL;

  fz_var(page);
  fz_var(device);
  fz_var(pixmap);
  fz_var(buffer);

  fz_try(ctx) {
    page = pdf_load_page(ctx, doc, params.page);
    dimension d = calculate_dimensions(ctx, page, params.width, params.scale, params.dpi);
    fz_irect bbox = fz_round_rect(d.bounds);
    pixmap = fz_new_pixmap_with_bbox(ctx, fz_device_rgb(ctx), bbox, NULL, 1);
    fz_clear_pixmap_with_value(ctx, pixmap, 0xff);
    device = fz_new_draw_device(ctx, d.ctm, pixmap);
    fz_enable_device_hints(ctx, device, FZ_NO_CACHE);
    pdf_run_page(ctx, page, device, fz_identity, params.cookie);
    buffer = fz_new_buffer_from_pixmap_as_png(ctx, pixmap, fz_default_color_params);
    output.payload_length = fz_buffer_storage(ctx, buffer, NULL);
    output.payload = je_malloc(sizeof(char)*output.payload_length);
    memcpy(output.payload, fz_string_from_buffer(ctx, buffer), output.payload_length);
  } fz_always(ctx) {
    fz_drop_buffer(ctx, buffer);
    fz_try(ctx) {
      fz_close_device(ctx, device);
    } fz_catch(ctx) {}
    fz_drop_device(ctx, device);
    fz_drop_pixmap(ctx, pixmap);
    fz_drop_page(ctx, (fz_page*)page);
  } fz_catch(ctx) {
    output.error = strdup(fz_caught_message(ctx));
  }

  return output;
}

save_to_png_output save_to_png(save_to_png_input input) {
  save_to_png_output output;
  output.payload = NULL;
  output.payload_length = 0;
  output.error = NULL;

  fz_context *ctx = fz_clone_context(global_ctx);
  if (ctx == NULL) {
    output.error = strdup("fail to create a context");
    return output;
  }

  fz_stream *stream = NULL;
  pdf_document *doc = NULL;

  fz_var(stream);
  fz_var(doc);

  fz_try(ctx) {
    stream = fz_open_memory(ctx, (const unsigned char *)input.payload, input.payload_length);
    doc = pdf_open_document_with_stream(ctx, stream);
    output = save_to_png_with_document(ctx, doc, input.params);
  } fz_always(ctx) {
    pdf_drop_document(ctx, doc);
    fz_drop_stream(ctx, stream);
  } fz_catch(ctx) {
    output.error = strdup(fz_caught_message(ctx));
  }
  fz_drop_context(ctx);

  return output;
}

char *strdup(const char *s1) {
  char *str;
  size_t size = strlen(s1) + 1;
  str = je_malloc(size);
  if (str) {
    memcpy(str, s1, size);
  }
  return str;
}

fz_stext_page *nitro_new_stext_page_from_page(fz_context *ctx, pdf_page *page, const fz_stext_options *options, save_to_html_params params) {
  fz_stext_page *text;
  fz_device *dev = NULL;

  fz_var(dev);

  if (page == NULL)
    return NULL;

  dimension d = calculate_dimensions(ctx, page, params.width, params.scale, params.dpi);
  text = fz_new_stext_page(ctx, d.bounds);
  fz_try(ctx) {
    dev = fz_new_stext_device(ctx, text, options);
    fz_run_page_contents(ctx, &(page->super), dev, d.ctm, NULL);
    fz_close_device(ctx, dev);
  } fz_always(ctx) {
    fz_drop_device(ctx, dev);
  } fz_catch(ctx) {
    fz_drop_stext_page(ctx, text);
    fz_rethrow(ctx);
  }

  return text;
}

fz_stext_page *nitro_new_stext_page_from_page_number(fz_context *ctx, pdf_document *doc, int number, const fz_stext_options *options, save_to_html_params params) {
  pdf_page *page;
  fz_stext_page *text = NULL;

  page = pdf_load_page(ctx, doc, number);
  fz_try(ctx)
    text = nitro_new_stext_page_from_page(ctx, page, options, params);
  fz_always(ctx)
    pdf_drop_page(ctx, page);
  fz_catch(ctx)
    fz_rethrow(ctx);

  return text;
}

save_to_html_output save_to_html(save_to_html_input input) {
  save_to_html_output output = {0};

  fz_context *ctx = fz_clone_context(global_ctx);
  if (ctx == NULL) {
    output.error = strdup("fail to create a context");
    return output;
  }

  fz_stream *stream = NULL;
  pdf_document *doc = NULL;
  fz_buffer *html_buffer = NULL;
  fz_output *out = NULL;
  fz_stext_page *text_page = NULL;

  fz_var(stream);
  fz_var(doc);
  fz_var(html_buffer);
  fz_var(out);
  fz_var(text_page);

  fz_try(ctx) {
    stream = fz_open_memory(ctx, (unsigned char *)input.payload, input.payload_length);
    doc = pdf_open_document_with_stream(ctx, stream);
    if (!doc) {
      fz_throw(ctx, FZ_ERROR_GENERIC, "failed to open document");
    }

    html_buffer = fz_new_buffer(ctx, 8192);
    out = fz_new_output_with_buffer(ctx, html_buffer);
    fz_write_string(ctx, out, "<!DOCTYPE html>\n<html>\n<head>\n<style>\np{position:absolute;white-space:pre;margin:0}\n</style>\n</head>\n<body>\n");

    fz_stext_options stext_options = { 0 };
    stext_options.flags |= FZ_STEXT_CLIP;
    stext_options.flags |= FZ_STEXT_ACCURATE_BBOXES;
    stext_options.flags |= FZ_STEXT_PRESERVE_WHITESPACE;
    stext_options.flags |= FZ_STEXT_COLLECT_STRUCTURE;
    stext_options.flags |= FZ_STEXT_COLLECT_VECTORS;
    text_page = nitro_new_stext_page_from_page_number(ctx, doc, input.params.page, &stext_options, input.params);

    fz_print_stext_page_as_html(ctx, out, text_page, input.params.page);
    fz_write_string(ctx, out, "</body></html>");
    fz_close_output(ctx, out);

    output.payload = je_malloc(html_buffer->len + 1);
    if (!output.payload) {
      fz_throw(ctx, FZ_ERROR_GENERIC, "failed to allocate memory for output");
    }

    memcpy(output.payload, html_buffer->data, html_buffer->len);
    output.payload[html_buffer->len] = '\0';
    output.payload_length = html_buffer->len;
  }
  fz_always(ctx) {
    if (text_page) fz_drop_stext_page(ctx, text_page);
    if (out) fz_drop_output(ctx, out);
    if (html_buffer) fz_drop_buffer(ctx, html_buffer);
    if (doc) pdf_drop_document(ctx, doc);
    if (stream) fz_drop_stream(ctx, stream);
  }
  fz_catch(ctx) {
    output.error = strdup(fz_caught_message(ctx));
  }

  fz_drop_context(ctx);
  return output;
}
