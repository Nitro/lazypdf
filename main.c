#include <pthread.h>
#include "main.h"
#include "_cgo_export.h"

void *page_count_runner(void *args) {
	PageCountInput *input = args;
	PageCountOutput output = { .id = input->id, .count = 0, .error = NULL };

	fz_context *ctx = NULL;
	ctx = fz_new_context(NULL, NULL, FZ_STORE_DEFAULT);
	if (ctx == NULL) {
		output.error = "fail to create a context";
		callbackPageCountOutput(&output);
		return NULL;
	}
	fz_register_document_handlers(ctx);

	fz_stream *stream = NULL;
	fz_document *doc = NULL;

	fz_var(stream);
	fz_var(doc);

	fz_try(ctx) {
		stream = fz_open_memory(ctx, input->payload, input->payload_length);
		doc = fz_open_document_with_stream(ctx, "document.pdf", stream);
		output.count = fz_count_pages(ctx, doc);
	} fz_catch(ctx) {
		output.error = fz_caught_message(ctx);
	}

	callbackPageCountOutput(&output);
	fz_drop_document(ctx, doc);
	fz_drop_stream(ctx, stream);
	fz_drop_context(ctx);
	return NULL;
}

void page_count(PageCountInput *input) {
	pthread_t thread;
	pthread_create(&thread, NULL, page_count_runner, input);
	pthread_detach(thread);
}

void *save_to_png_runner(void *args) {
	SaveToPNGInput *input = args;
	SaveToPNGOutput output = { .id = input->id, .data = NULL, .len = 0, .error = NULL};
	fz_context *ctx = fz_new_context(NULL, NULL, FZ_STORE_DEFAULT);
	fz_register_document_handlers(ctx);
	fz_stream *stream = NULL;
	fz_document *doc = NULL;
	fz_page *page = NULL;
	fz_display_list *list = NULL;
	fz_device *device = NULL;
	fz_pixmap *pixmap = NULL;
	fz_device *draw_device = NULL;
	fz_buffer *buffer = NULL;
	fz_try(ctx) {
		stream = fz_open_memory(ctx, input->payload, input->payload_length);
		doc = fz_open_document_with_stream(ctx, "document.pdf", stream);
		page = fz_load_page(ctx, doc, input->page);

		float scale_factor = 1.5;
		fz_rect bounds = fz_bound_page(ctx, page);
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
		list = fz_new_display_list(ctx, bounds);
		device = fz_new_list_device(ctx, list);
		fz_run_page(ctx, page, device, fz_identity, NULL);
		pixmap = fz_new_pixmap_with_bbox(ctx, fz_device_rgb(ctx), bbox, NULL, 1);
		fz_clear_pixmap_with_value(ctx, pixmap, 0xff);
		draw_device = fz_new_draw_device(ctx, ctm, pixmap);
		fz_run_display_list(ctx, list, draw_device, fz_identity, bounds, NULL);
		buffer = fz_new_buffer_from_pixmap_as_png(ctx, pixmap, fz_default_color_params);

		size_t len = fz_buffer_storage(ctx, buffer, NULL);
		const char *result = fz_string_from_buffer(ctx, buffer);
		output.len = len;
		output.data = (char *)(result);
	}
	fz_catch(ctx) {
		output.error = fz_caught_message(ctx);
	}

	callbackSaveToPNGOutput(&output);
	fz_drop_buffer(ctx, buffer);
	fz_close_device(ctx, draw_device);
	fz_drop_device(ctx, draw_device);
	fz_drop_pixmap(ctx, pixmap);
	fz_drop_device(ctx, device);
	fz_drop_display_list(ctx, list);
	fz_drop_page(ctx, page);
	fz_drop_document(ctx, doc);
	fz_drop_stream(ctx, stream);
	fz_drop_context(ctx);
	return NULL;
}

void save_to_png(SaveToPNGInput *input) {
	pthread_t thread;
	pthread_create(&thread, NULL, save_to_png_runner, input);
}
