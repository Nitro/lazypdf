#include <stddef.h>
#include <stdlib.h>

// indent -linux -br -brf

#define DLL_EXPORT

typedef void * document_handle;

DLL_EXPORT document_handle open_document(const char* in_file);
DLL_EXPORT void close_document(document_handle doc);

DLL_EXPORT size_t get_num_pages(document_handle doc);

// int here is return error code, 0 = sucess
DLL_EXPORT int get_page_dimensions(document_handle doc, int32_t page, double zoom, uint32_t* width, uint32_t* height);

// returns number of bytes copied in buffer
DLL_EXPORT size_t render_page(document_handle doc, int32_t page, double zoom, char* buffer, size_t buf_len);