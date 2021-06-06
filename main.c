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
		output->error = "fail to create a context";
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
		output->error = fz_caught_message(ctx);
	}
	fz_drop_context(ctx);

	return output;
}

save_to_png_output *save_to_png(save_to_png_input *input) {
	save_to_png_output *output = malloc(sizeof(save_to_png_output));
	output->ctx = NULL;
	output->buffer = NULL;
	output->data = NULL;
	output->len = 0;
	output->error = NULL;

	output->ctx = fz_new_context(NULL, NULL, FZ_STORE_DEFAULT);
	if (output->ctx == NULL) {
		output->error = "fail to create a context";
		return output;
	}
	fz_register_document_handlers(output->ctx);

	fz_stream *stream = NULL;
	pdf_document *doc = NULL;
	pdf_page *page = NULL;
	fz_display_list *list = NULL;
	fz_device *device = NULL;
	fz_pixmap *pixmap = NULL;
	fz_device *draw_device = NULL;

	fz_var(stream);
	fz_var(doc);
	fz_var(page);
	fz_var(list);
	fz_var(device);
	fz_var(pixmap);
	fz_var(draw_device);

	fz_try(output->ctx) {
		stream = fz_open_memory(output->ctx, input->payload, input->payload_length);
		doc = pdf_open_document_with_stream(output->ctx, stream);
		page = pdf_load_page(output->ctx, doc, input->page);

		float scale_factor = 1.5;
		fz_rect bounds = pdf_bound_page(output->ctx, page);
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
		list = fz_new_display_list(output->ctx, bounds);
		device = fz_new_list_device(output->ctx, list);
		fz_enable_device_hints(output->ctx, device, FZ_NO_CACHE);
		pdf_run_page(output->ctx, page, device, fz_identity, NULL);
		pixmap = fz_new_pixmap_with_bbox(output->ctx, fz_device_rgb(output->ctx), bbox, NULL, 1);
		fz_clear_pixmap_with_value(output->ctx, pixmap, 0xff);
		draw_device = fz_new_draw_device(output->ctx, ctm, pixmap);
		fz_run_display_list(output->ctx, list, draw_device, fz_identity, bounds, NULL);
		output->buffer = fz_new_buffer_from_pixmap_as_png(output->ctx, pixmap, fz_default_color_params);

		size_t len = fz_buffer_storage(output->ctx, output->buffer, NULL);
		const char *result = fz_string_from_buffer(output->ctx, output->buffer);
		output->len = len;
		output->data = (char *)(result);
	} fz_always(output->ctx) {
		fz_close_device(output->ctx, draw_device);
		fz_drop_device(output->ctx, draw_device);
		fz_drop_pixmap(output->ctx, pixmap);
		fz_drop_device(output->ctx, device);
		fz_drop_display_list(output->ctx, list);
		fz_drop_page(output->ctx, (fz_page*)page);
		pdf_drop_document(output->ctx, doc);
		fz_drop_stream(output->ctx, stream);
	} fz_catch(output->ctx) {
		output->error = fz_caught_message(output->ctx);
	}

	return output;
}
