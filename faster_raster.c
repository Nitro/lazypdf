#include "faster_raster.h"

#include <stdio.h>
// #include <string.h>

// Format with:
// indent -linux -br -brf

DLL_EXPORT document_handle open_document(const char* in_file) {
	fprintf(stdout, "Opening document '%s'\n", in_file);
	return (void *)42;
}

DLL_EXPORT void close_document(document_handle doc) {
	fprintf(stdout, "Closing document\n");
	// TODO: We might want to return some error code from here...
}

DLL_EXPORT size_t get_num_pages(document_handle doc) {
	fprintf(stdout, "Getting number of pages\n");
	return 42;
}

// int here is return error code, 0 = sucess
DLL_EXPORT int get_page_dimensions(document_handle doc, int32_t page, double zoom, uint32_t* width, uint32_t* height) {
	fprintf(stdout, "Getting page dimensions\n");
	*width = 1024;
	*height = 768;
	return 0;
}

// returns number of bytes copied in buffer
DLL_EXPORT size_t render_page(document_handle doc, int32_t page, double zoom, char* buffer, size_t buf_len) {
	fprintf(stdout, "Rendering page %d\n", page);
	return 666;
}