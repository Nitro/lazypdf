#ifndef MAIN_H
#define MAIN_H

#include "pdf.h"

typedef struct {
	char *payload;
	size_t payload_length;
} page_count_input;

typedef struct {
	int count;
	char *error;
} page_count_output;

typedef struct {
	int page;
	int width;
	float scale;
	int dpi;
	fz_cookie *cookie;
} save_to_png_params;

typedef struct {
	save_to_png_params params;
	char *payload;
	size_t payload_length;
} save_to_png_input;

typedef struct {
	char *payload;
	size_t payload_length;
	char *error;
} save_to_png_output;

typedef struct {
	int page;
	int width;
	float scale;
	int dpi;
	fz_cookie *cookie;
} save_to_html_params;

typedef struct {
	save_to_html_params params;
	char *payload;
	size_t payload_length;
} save_to_html_input;

typedef struct {
	char *payload;
	size_t payload_length;
	char *error;
} save_to_html_output;

void init();

page_count_output page_count(page_count_input input);
save_to_png_output save_to_png(save_to_png_input input);
save_to_png_output save_to_png_with_document(fz_context *ctx, pdf_document *doc, save_to_png_params params);
save_to_html_output save_to_html(save_to_html_input input);

#endif
