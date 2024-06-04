#include <jemalloc/jemalloc.h>
#include <pthread.h>
#include <string.h>
#include "main.h"

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
	pdf_page *page = NULL;
	fz_device *device = NULL;
	fz_pixmap *pixmap = NULL;
	fz_buffer *buffer = NULL;

	fz_var(stream);
	fz_var(doc);
	fz_var(page);
	fz_var(device);
	fz_var(pixmap);
	fz_var(buffer);

	fz_try(ctx) {
		stream = fz_open_memory(ctx, (const unsigned char *)input.payload, input.payload_length);
		doc = pdf_open_document_with_stream(ctx, stream);
		page = pdf_load_page(ctx, doc, input.page);

		float scale_factor = 1.5;
		fz_rect bounds = pdf_bound_page(ctx, page);
		if (input.width != 0) {
			scale_factor = input.width / bounds.x1;
		} else if (input.scale != 0) {
			scale_factor = input.scale;
		} else if ((bounds.x1 - bounds.x0) > (bounds.y1 - bounds.y0)) {
			switch (get_rotation(ctx, page)) {
				case 0:
				case 180:
					scale_factor = 1;
					break;
				default:
					scale_factor = 1.5;
			}
		}

		float resolution = (float)(input.dpi) / 72;
		fz_matrix ctm = fz_concat(fz_scale(resolution, resolution), fz_scale(scale_factor, scale_factor));
		bounds = fz_transform_rect(bounds, ctm);
		fz_irect bbox = fz_round_rect(bounds);
		pixmap = fz_new_pixmap_with_bbox(ctx, fz_device_rgb(ctx), bbox, NULL, 1);
		fz_clear_pixmap_with_value(ctx, pixmap, 0xff);
		device = fz_new_draw_device(ctx, ctm, pixmap);
		fz_enable_device_hints(ctx, device, FZ_NO_CACHE);
		pdf_run_page(ctx, page, device, fz_identity, input.cookie);
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
