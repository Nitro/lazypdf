#ifndef MAIN_H
#define MAIN_H

#include "pdf.h"

typedef const char cchar;

typedef struct {
	fz_context *context;
	fz_buffer *buffer;
	size_t len;
	const char *data;
} result;

result *save_to_png(int page_number, int width, float scale, const unsigned char *payload, size_t payload_length);
void drop_result(result *r);

#endif