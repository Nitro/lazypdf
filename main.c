#include <string.h>
#include "main.h"
#include "_cgo_export.h"

page_count_output *page_count(page_count_input *input) {
	page_count_output *output = malloc(sizeof(page_count_output));
	output->count = 0;
	output->error = NULL;

	fz_context *ctx = NULL;
	ctx = fz_new_context(NULL, NULL, FZ_STORE_DEFAULT);
	if (ctx == NULL) {
		output->error = strdup("fail to create a context");
		return output;
	}
	fz_register_document_handlers(ctx);

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

	fz_context *ctx = NULL;
	ctx = fz_new_context(NULL, NULL, FZ_STORE_DEFAULT);
	if (ctx == NULL) {
		output->error = strdup("fail to create a context");
		return output;
	}
	fz_register_document_handlers(ctx);

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
		output->len = fz_buffer_extract(ctx, buffer, (unsigned char **)&output->data);
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
