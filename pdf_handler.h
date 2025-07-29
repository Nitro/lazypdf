#ifndef PDF_HANDLER_H
#define PDF_HANDLER_H

#include <stdint.h>
#include <string.h>
#include <stdlib.h>
#include "mupdf/fitz.h"
#include "mupdf/pdf.h"


typedef struct {
    char *filename;
} openPDFInput;

typedef struct {
    uintptr_t handle;
    char *error;
} pdfDocument;


pdfDocument open_pdf(openPDFInput);


typedef struct {
    char *error; // NULL if successful
} closePDFOutput;

closePDFOutput close_pdf(pdfDocument);

typedef struct {
    float width;
    float height;
    char *error; 
} PageSizeOutput;

PageSizeOutput get_page_size(pdfDocument, int);

typedef struct {
    int page;
    const char *path;
    float x;
    float y;
    float width;
    float height;
} addImageInput;

typedef struct {
    char *error; 
} addImageOutput;

addImageOutput add_image_to_page(pdfDocument,  addImageInput);

typedef struct {
    const char *text;
    int page;
    float x;
    float y;
    const char *font_family;
    const char *font_path;
    float font_size;
} addTextInput;

typedef struct {
    char *error; // NULL if successful
} addTextOutput;

addTextOutput add_text_to_page(pdfDocument document, addTextInput input);

typedef struct {
    int value; 
    int page;
    float x;
    float y;
    float width;
    float height;
} addCheckboxInput;

typedef struct {
    char *error; // NULL if successful
} addCheckboxOutput;

addCheckboxOutput add_checkbox_to_page(pdfDocument document, addCheckboxInput input);

typedef struct {
    char *error; // NULL if successful
} savePDFOutput;

savePDFOutput save_pdf(pdfDocument document, const char *file_path);

typedef struct {
    int page;
    int width;
    float scale;
    int dpi;
	fz_cookie *cookie;
} saveToPNGInput;

typedef struct {
    char *payload;
    size_t payload_length;
    char *error; // NULL if successful
} saveToPNGOutput;

saveToPNGOutput save_to_png_file(pdfDocument document, saveToPNGInput input);

#endif
