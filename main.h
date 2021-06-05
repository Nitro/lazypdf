#ifndef MAIN_H
#define MAIN_H

#include "pdf.h"

typedef struct {
	char *id;
	int page;
	int width;
	float scale;
	const unsigned char *payload;
	size_t payload_length;
} SaveToPNGInput;

typedef struct {
	char *id;
	char *data;
	size_t len;
	const char *error;
} SaveToPNGOutput;

typedef struct {
	char *id;
	const unsigned char *payload;
	size_t payload_length;
} PageCountInput;

typedef struct {
	char *id;
	int count;
	const char *error;
} PageCountOutput;

void page_count(PageCountInput *input);
void save_to_png(SaveToPNGInput *input);

#endif