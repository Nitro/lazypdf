#include <pthread.h>
#include <string.h>
#include "main.h"

fz_context *global_ctx;
fz_locks_context *global_ctx_lock;
pthread_mutex_t *global_ctx_mutex;

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

void init(size_t lock_quantity) {
	global_ctx_mutex = malloc(sizeof(pthread_mutex_t) * lock_quantity);

	for (size_t i = 0; i < lock_quantity; i++) {
		if (pthread_mutex_init(&global_ctx_mutex[i], NULL) != 0) {
			fail("pthread_mutex_init()");
		}
	}

	global_ctx_lock = malloc(sizeof(fz_locks_context));
	global_ctx_lock->user = global_ctx_mutex;
	global_ctx_lock->lock = lock_mutex;
	global_ctx_lock->unlock = unlock_mutex;

	global_ctx = fz_new_context(NULL, global_ctx_lock, FZ_STORE_DEFAULT);
	fz_register_document_handlers(global_ctx);
}

page_count_output *page_count(page_count_input *input) {
	page_count_output *output = malloc(sizeof(page_count_output));
	output->count = 0;
	output->error = NULL;

	fz_context *ctx = fz_clone_context(global_ctx);
	if (ctx == NULL) {
		output->error = strdup("fail to create a context");
		return output;
	}

	fz_stream *stream = NULL;
	pdf_document *doc = NULL;

	fz_var(stream);
	fz_var(doc);

	fz_try(ctx) {
		stream = fz_open_memory(ctx, input->payload, input->payload_length);
		doc = pdf_open_document_with_stream(ctx, stream);
		output->count = pdf_count_pages(ctx, doc);
	} fz_always(ctx) {
		pdf_drop_document(ctx, doc);
		fz_drop_stream(ctx, stream);
  } fz_catch(ctx) {
		output->error = strdup(fz_caught_message(ctx));
	}
	fz_drop_context(ctx);

	return output;
}

save_to_png_output *save_to_png(save_to_png_input *input) {
	save_to_png_output *output = malloc(sizeof(save_to_png_output));
	output->data = NULL;
	output->len = 0;
	output->error = NULL;

	fz_context *ctx = fz_clone_context(global_ctx);
	if (ctx == NULL) {
		output->error = strdup("fail to create a context");
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
		stream = fz_open_memory(ctx, input->payload, input->payload_length);
		doc = pdf_open_document_with_stream(ctx, stream);
		page = pdf_load_page(ctx, doc, input->page);

		float scale_factor = 1.5;
		fz_rect bounds = pdf_bound_page(ctx, page);
		if (input->width != 0) {
			scale_factor = input->width / bounds.x1;
		} else if (input->scale != 0) {
			scale_factor = input->scale;
		} else if ((bounds.x1 - bounds.x0) > (bounds.y1 - bounds.y0)) {
			scale_factor = 1;
		}

		fz_matrix ctm = fz_scale(scale_factor, scale_factor);
		bounds = fz_transform_rect(bounds, ctm);
		fz_irect bbox = fz_round_rect(bounds);
		pixmap = fz_new_pixmap_with_bbox(ctx, fz_device_rgb(ctx), bbox, NULL, 1);
		fz_clear_pixmap_with_value(ctx, pixmap, 0xff);
		device = fz_new_draw_device(ctx, ctm, pixmap);
		fz_enable_device_hints(ctx, device, FZ_NO_CACHE);
		pdf_run_page(ctx, page, device, fz_identity, NULL);
		buffer = fz_new_buffer_from_pixmap_as_png(ctx, pixmap, fz_default_color_params);
		output->len = fz_buffer_storage(ctx, buffer, NULL);
		output->data = malloc(sizeof(char)*output->len);
		memcpy(output->data, fz_string_from_buffer(ctx, buffer), output->len);
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
		output->error = strdup(fz_caught_message(ctx));
	}
	fz_drop_context(ctx);

	return output;
}
