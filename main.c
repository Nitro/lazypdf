#include <string.h>
#include "main.h"
#include "_cgo_export.h"

result *save_to_png(int page_number, int width, float scale, const unsigned char *payload, size_t payload_length) {
	fz_context *ctx = fz_new_context(NULL, NULL, FZ_STORE_DEFAULT);
	if (!ctx) {
		errorHandler("fail to create mupdf context");
		return NULL;
	}

	result *r = malloc(sizeof(result));
	r->context = ctx;
	r->buffer = NULL;

	int exit = 0;
	fz_stream *stream = NULL;
	fz_document *doc = NULL;
	fz_page *page = NULL;
	fz_display_list *list = NULL;
	fz_device *device = NULL;
	fz_pixmap *pixmap = NULL;
	fz_device *draw_device = NULL;
	fz_buffer *buffer = NULL;
	fz_try(ctx) {
		fz_register_document_handlers(ctx);
		stream = fz_open_memory(ctx, payload, payload_length);
		doc = fz_open_document_with_stream(ctx, "document.pdf", stream);
		page = fz_load_page(ctx, doc, page_number);

		float scale_factor = 1.5;
		fz_rect bounds = fz_bound_page(ctx, page);
		if (width != 0) {
			scale_factor = width / bounds.x1;
		} else if (scale != 0) {
			scale_factor = scale;
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
		r->buffer = buffer;

		size_t len = fz_buffer_storage(ctx, buffer, NULL);
		const char *output = fz_string_from_buffer(ctx, buffer);
		r->len = len;
		r->data = output;
	}
	fz_catch(ctx) {
		errorHandler(fz_caught_message(ctx));
		exit = 1;
	}

	fz_try(ctx)
		fz_close_device(ctx, draw_device);
	fz_catch(ctx) {
		errorHandler(fz_caught_message(ctx));
		exit = 1;
	}
	fz_drop_device(ctx, draw_device);
	fz_drop_pixmap(ctx, pixmap);
	fz_drop_device(ctx, device);
	fz_drop_display_list(ctx, list);
	fz_drop_page(ctx, page);
	fz_drop_document(ctx, doc);
	fz_drop_stream(ctx, stream);

	if (exit == 1) {
		drop_result(r);
		return NULL;
	}
	return r;
}

void drop_result(result *r) {
	fz_drop_buffer(r->context, r->buffer);
	fz_drop_context(r->context);
	free(r);
};
