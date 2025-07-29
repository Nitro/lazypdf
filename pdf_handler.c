#include <pthread.h>
#include <stdbool.h>

#include <mupdf/fitz.h>
#include <mupdf/pdf.h>

#include "pdf_handler.h"

extern fz_context *global_ctx;

pdfDocument open_pdf(openPDFInput input) {
    pdfDocument output  = {.handle = 0, .error = NULL };
    fz_stream *stream   = NULL;
    pdf_document *doc   = NULL;

    fz_context *ctx = fz_clone_context(global_ctx);
    if (ctx == NULL) {
        output.error = strdup("fail to clone a context");
        return output;
    }

    fz_try(ctx) {
        doc = pdf_open_document(ctx, input.filename);
        output.handle = (uintptr_t)doc;
    } fz_catch(ctx) {
        output.error = strdup(fz_caught_message(ctx));
    }
    fz_drop_context(ctx);

    return output;
}

closePDFOutput close_pdf(pdfDocument input) {
    closePDFOutput output;
    output.error = NULL;

    fz_context *ctx = fz_clone_context(global_ctx);
    if (ctx == NULL) {
        output.error = strdup("fail to clone a context");
        return output;
    }

    fz_try(ctx) {
        pdf_document *doc = (pdf_document *)input.handle;
        pdf_drop_document(ctx, doc);
    } fz_catch(ctx) {
        output.error = strdup(fz_caught_message(ctx));
    }
    fz_drop_context(ctx);
    return output;
}

int page_insert_content_to_content_stream(fz_context *ctx, pdf_page *page, fz_buffer *content, bool append) {
    pdf_obj *existing_content = pdf_dict_get(ctx, page->obj, PDF_NAME(Contents));
    pdf_obj *new_stream = pdf_add_stream(ctx, page->doc, content, NULL, 0);
    int stream_num = pdf_to_num(ctx, new_stream);

    if (pdf_is_array(ctx, existing_content)) {
        if (append)
            pdf_array_push(ctx, existing_content, new_stream);
        else
            pdf_array_insert(ctx, existing_content, new_stream, 0);        
    }
    else {
        pdf_obj *array = pdf_new_array(ctx, page->doc, 5);
        if (existing_content && pdf_to_num(ctx, existing_content)) {
            pdf_array_push(ctx, array, existing_content);
        }
        if (append) {
            pdf_array_push(ctx, array, new_stream);
        } else {
            pdf_array_insert(ctx, array, new_stream, 0);
        }
        pdf_dict_put(ctx, page->obj, PDF_NAME(Contents), array);
    }
    return stream_num;
}

int page_add_content_to_content_stream(fz_context *ctx, pdf_page *page, fz_buffer *content) {
    return page_insert_content_to_content_stream(ctx, page, content, true);
}


void wrap_page_contents(fz_context *ctx, pdf_page *page) {
    // Ensure page is in a balanced graphics state
    pdf_obj *resources = NULL;
    pdf_obj *contents = NULL;
    fz_buffer *buf = NULL;

    int prepend = 0;
    int append = 0;
    
    resources = pdf_dict_get(ctx, page->obj, PDF_NAME(Resources));
    contents = pdf_dict_get(ctx, page->obj, PDF_NAME(Contents));

    pdf_count_q_balance(ctx, page->doc, resources, contents, &prepend, &append);

    if (prepend)
    {
        // Prepend enough 'q' to ensure we can get back to initial state.
        buf = fz_new_buffer(ctx, 1024);
        while (prepend-- > 0)
            fz_append_string(ctx, buf, "q\n");
        page_insert_content_to_content_stream(ctx, page, buf, false);
        fz_drop_buffer(ctx, buf);
    }

    if (append)
    {
        // Append enough 'Q' to ensure we can get back to initial state.
        buf = fz_new_buffer(ctx, 1024);
        while (append-- > 0)
            fz_append_string(ctx, buf, "Q\n");
        page_add_content_to_content_stream(ctx, page, buf);
        fz_drop_buffer(ctx, buf);
    }
}

pdf_obj *get_or_create_dict(fz_context *ctx, pdf_obj *parent, pdf_obj *key) {
    pdf_obj *dict = pdf_dict_get(ctx, parent, key);
    if (!dict) {
        dict = pdf_dict_put_dict(ctx, parent, key, 2);
        if (!dict) {
            fz_throw(ctx, FZ_ERROR_GENERIC, "Failed to get or create dictionary for key: %s", pdf_to_name(ctx, key));
        }
    }
    return dict;
}

int  page_get_rotation(fz_context *ctx, pdf_page *page) {
    int rotation = 0;
    rotation = pdf_dict_get_inheritable_int(ctx, page->obj, PDF_NAME(Rotate));
    if (rotation < 0) {
        rotation = 360 - ((-rotation) % 360);
    }
    if (rotation >= 360) {
        rotation = rotation % 360;
    }
    rotation = 90*((rotation + 45)/90);
    if (rotation >= 360) {
        rotation = 0;
    }
    return rotation;
}

fz_matrix rect_to_page_space(fz_context *ctx, pdf_page *page, fz_rect position) {
    fz_matrix transform         = fz_identity;
    fz_matrix page_matrix       = fz_identity;
    fz_rect page_crop_box       = fz_empty_rect;
    fz_point page_crop_offset   = { .x = 0, .y = 0 };
    int rotation                = 0;
    float page_width            = 0;
    float page_height           = 0;

    pdf_page_transform(ctx, page, &page_crop_box, &page_matrix);
    page_crop_offset = fz_make_point(page_crop_box.x0, page_crop_box.y0);
    page_crop_box = fz_transform_rect(page_crop_box, page_matrix);

    page_width  = page_crop_box.x1 - page_crop_box.x0;
    page_height = page_crop_box.y1 - page_crop_box.y0;


    rotation = page_get_rotation(ctx, page);
    transform = fz_concat(transform, fz_rotate(rotation));

    float width = position.x1 - position.x0;
    float height = position.y1 - position.y0;

    switch (rotation) {
        case 0:
            transform = fz_concat(transform, fz_scale(width, height));
            transform = fz_concat(transform, fz_translate(position.x0, position.y0));
            break;
        case 90:
            transform = fz_concat(transform, fz_scale(height, width));
            transform = fz_concat(transform, fz_translate(page_height - position.y0, position.x0));
            break;
        case 180:
            transform = fz_concat(transform, fz_scale(width, height));
            transform = fz_concat(transform, fz_translate(page_width - position.x0, page_height - position.y0));
            break;
        case 270:
            transform = fz_concat(transform, fz_scale(height, width));
            transform = fz_concat(transform, fz_translate(position.y0, page_width - position.x0));
            break;
        default:
            break;
    }
    transform = fz_concat(transform, fz_translate(page_crop_offset.x, page_crop_offset.y));
    return transform;
}

fz_matrix point_to_page_space(fz_context *ctx, pdf_page *page, fz_point position) {
    fz_matrix transform         = fz_identity;
    fz_matrix page_matrix       = fz_identity;
    fz_rect page_crop_box       = fz_empty_rect;
    fz_point page_crop_offset   = { .x = 0, .y = 0 };
    int rotation                = 0;
    float page_width            = 0;
    float page_height           = 0;

    pdf_page_transform(ctx, page, &page_crop_box, &page_matrix);
    page_crop_offset = fz_make_point(page_crop_box.x0, page_crop_box.y0);
    page_crop_box = fz_transform_rect(page_crop_box, page_matrix);

    page_width  = page_crop_box.x1 - page_crop_box.x0;
    page_height = page_crop_box.y1 - page_crop_box.y0;

    rotation = page_get_rotation(ctx, page);
    transform = fz_concat(transform, fz_rotate(rotation));

    switch (rotation) {
        case 0:
            transform = fz_concat(transform, fz_translate(position.x, position.y));
            break;
        case 90:
            transform = fz_concat(transform, fz_translate(page_height - position.y, position.x));
            break;
        case 180:
            transform = fz_concat(transform, fz_translate(page_width - position.x, page_height - position.y));
            break;
        case 270:
            transform = fz_concat(transform, fz_translate(position.y, page_width - position.x));
            break;
    }
    transform = fz_concat(transform, fz_translate(page_crop_offset.x, page_crop_offset.y));
    return transform;
}

PageSizeOutput get_page_size(pdfDocument document, int page_number) {
    PageSizeOutput output   = { .error = NULL, .width=0, .height=0 };
    pdf_document *pdf = NULL;
    pdf_page *page = NULL;
    fz_context *ctx = fz_clone_context(global_ctx);
    if (!ctx) {
        output.error = strdup("Context clone failed");
        return output;
    }
    fz_var(pdf);
    fz_try(ctx) {
        pdf  = (pdf_document *)document.handle;
        page = pdf_load_page(ctx, pdf, page_number);
        fz_rect page_crop_box = pdf_bound_page(ctx, page, FZ_CROP_BOX);
        output.width = page_crop_box.x1 - page_crop_box.x0;
        output.height = page_crop_box.y1 - page_crop_box.y0;

    }
    fz_always(ctx) {
        if (page != NULL) {
            fz_drop_page(ctx, &page->super);
        }
    }
    fz_catch(ctx) {
        output.error = strdup(fz_caught_message(ctx));
    }

    fz_drop_context(ctx);
    return output;
}

void page_add_image(fz_context *ctx, pdf_page *page, fz_image *image, fz_rect position) {
    pdf_obj *resources      = NULL;
    pdf_obj *xobject        = NULL;
    pdf_obj *image_object   = NULL;
    fz_buffer *stream       = NULL;
    fz_matrix matrix        = fz_identity;
    char resource_name[32]  = "";

    fz_var(image_object);
    fz_var(stream);

    fz_try(ctx) {
        // Add image to Resources
        resources = get_or_create_dict(ctx, page->obj, PDF_NAME(Resources));
        xobject = get_or_create_dict(ctx, resources, PDF_NAME(XObject));
        image_object = pdf_add_image(ctx, page->doc, image);
        if (!image_object) {
            fz_throw(ctx, FZ_ERROR_GENERIC, "Failed to add image to document");
        }
        fz_snprintf(resource_name, sizeof(resource_name), "Img%d", pdf_to_num(ctx, image_object));
        pdf_dict_puts(ctx, xobject, resource_name, image_object);
        // Add image to content stream
        stream = fz_new_buffer(ctx, 1024);
        matrix = rect_to_page_space(ctx, page, position);
        fz_append_string(ctx, stream, "q\n");               // Saves the current graphics state.
        fz_append_printf(ctx, stream, "%g %g %g %g %g %g cm\n",
            matrix.a,
            matrix.b,
            matrix.c,
            matrix.d,
            matrix.e,
            matrix.f
        );                                                  // Set the transformation matrix for positioning, scaling and rotating
        fz_append_printf(ctx, stream, "/%s Do\n",
           resource_name
        );                                                  // Draw Image
        fz_append_string(ctx, stream, "Q\n");               // Restores the previously saved graphics state

        page_add_content_to_content_stream(ctx, page, stream);
    }
    fz_always(ctx) {
        fz_drop_buffer(ctx, stream);
        pdf_drop_obj(ctx, image_object);
    }
    fz_catch(ctx) {
        fz_rethrow(ctx);
    }
}

addImageOutput add_image_to_page(pdfDocument document, addImageInput input) {
    addImageOutput output   = { .error = NULL };
    pdf_document *pdf       = NULL;
    pdf_page *page          = NULL;
    fz_image *image         = NULL;
    fz_rect position        = {
        input.x,
        input.y,
        input.x + input.width,
        input.y + input.height
    };

    fz_context *ctx = fz_clone_context(global_ctx);
    if (!ctx) {
        output.error = strdup("Context clone failed");
        return output;
    }

    fz_var(pdf);
    fz_var(image);
    fz_try(ctx) {
        pdf       = (pdf_document *)document.handle;
        page = pdf_load_page(ctx, pdf, input.page);
        image = fz_new_image_from_file(ctx, input.path);
        if (!image) {
            fz_throw(ctx, FZ_ERROR_GENERIC, "Failed to load image from %s", input.path);
        }

        wrap_page_contents(ctx, page);
        page_add_image(ctx, page, image, position);
    }
    fz_always(ctx) {
        fz_drop_image(ctx, image);
        if (page != NULL) {
            fz_drop_page(ctx, &page->super);
        }
    }
    fz_catch(ctx) {
        output.error = strdup(fz_caught_message(ctx));
    }

    fz_drop_context(ctx);
    return output;
}

void page_add_text(fz_context *ctx, pdf_page *page, const char *text, fz_point position, fz_font *font, float font_size, const char *encoding_name) {
    fz_buffer *stream           = NULL;
    pdf_obj *resources          = NULL;
    pdf_obj *font_dict          = NULL;
    pdf_obj *font_ref           = NULL;
    char resource_name[32]      = "";
    int encoding                = PDF_SIMPLE_ENCODING_LATIN;
    fz_matrix matrix            = fz_identity;
    size_t max_length           = 300;

    fz_var(stream);
    fz_var(font_ref);

    fz_try(ctx) {

        size_t text_length  = strlen(text);
        if (text_length > max_length) {
            fz_throw(ctx, FZ_ERROR_GENERIC, "Text exceeds maximum allowed size. Expected: %zu, Actual: %zu", max_length, text_length);
        }

        stream = fz_new_buffer(ctx, text_length  + 500);

        // Add font to Resources
        resources   = get_or_create_dict(ctx, page->obj, PDF_NAME(Resources));
        font_dict   = get_or_create_dict(ctx, resources, PDF_NAME(Font));

        if (encoding_name) {
            if (!strcmp(encoding_name, "Latin")) {
                encoding = PDF_SIMPLE_ENCODING_LATIN;
            }
            else if (!strcmp(encoding_name, "Greek")) {
                encoding = PDF_SIMPLE_ENCODING_GREEK;
            }
            else if (!strcmp(encoding_name, "Cyrillic")) {
                encoding = PDF_SIMPLE_ENCODING_CYRILLIC;
            }
        }
        font_ref = pdf_add_simple_font(ctx, page->doc, font, encoding);
        fz_snprintf(resource_name, sizeof(resource_name), "Font%d", pdf_to_num(ctx, font_ref));
        pdf_dict_puts(ctx, font_dict, resource_name, font_ref);

        // Add text to content stream
        matrix = point_to_page_space(ctx, page, position);
        fz_append_string(ctx, stream, "q\n");               // Saves the current graphics state.
        fz_append_printf(ctx, stream, "%g %g %g %g %g %g cm\n",
            matrix.a,
            matrix.b,
            matrix.c,
            matrix.d,
            matrix.e,
            matrix.f
        );                                                  // Set the transformation matrix for positioning and rotating
        fz_append_string(ctx, stream, "BT\n");              // Begins a text object
        fz_append_printf(ctx, stream, "/%s %g Tf\n",
            resource_name,
            font_size
        );                                                  // Sets the font
        fz_append_string(ctx, stream, "0 0 Td\n");          // Set the text position
        fz_append_printf(ctx, stream, "(%s) Tj\n",
            text
        );                                                  // Draw text
        fz_append_string(ctx, stream, "ET\n");              // Ends the text object.
        fz_append_string(ctx, stream, "Q\n");               // Restores the previously saved graphics state
        page_add_content_to_content_stream(ctx, page, stream);
    }
    fz_always(ctx) {
        fz_drop_buffer(ctx, stream);
        pdf_drop_obj(ctx, font_ref);
    }
    fz_catch(ctx) {
        fz_rethrow(ctx);
    }
}

addTextOutput add_text_to_page(pdfDocument document, addTextInput input) {
    addTextOutput output        = { .error = NULL };
    pdf_document *pdf           = NULL;
    pdf_page *page              = NULL;
    fz_font *font               = NULL;
    const unsigned char *data   = NULL;
    int size                    = 0;

    fz_context *ctx = fz_clone_context(global_ctx);
    if (ctx == NULL) {
        output.error = strdup("Failed to clone global context");
        return output;
    }

    fz_var(pdf);
    fz_var(font);

    fz_try(ctx) {
        pdf = (pdf_document *)document.handle;
        page = pdf_load_page(ctx, pdf, input.page);
        data = fz_lookup_base14_font(ctx, input.font_family, &size);
        if (data) {
            font = fz_new_font_from_memory(ctx, input.font_family, data, size, 0, 0);
        }
        else {
            font = fz_new_font_from_file(ctx, NULL, input.font_path, 0, 0);
        }
        if (!font) {
            fz_throw(ctx, FZ_ERROR_GENERIC, "Failed to load font %s %s", input.font_family, input.font_path);
        }
        fz_point position = {input.x, input.y};

        wrap_page_contents(ctx, page);
        page_add_text(ctx, page, input.text, position,font, input.font_size, "Latin");
    }
    fz_always(ctx) {
        fz_drop_font(ctx, font);
        if (page != NULL) {
            fz_drop_page(ctx, &page->super);
        }
    }
    fz_catch(ctx) {
        output.error = strdup(fz_caught_message(ctx));
    }

    fz_drop_context(ctx);
    return output;
}

void page_add_checkbox(fz_context *ctx, pdf_page *page, fz_rect position, int is_checked) {
    fz_buffer *stream           = NULL;
    fz_matrix matrix            = fz_identity;
    fz_font *font               = NULL;
    pdf_obj *resources          = NULL;
    pdf_obj *font_dict          = NULL;
    pdf_obj *font_ref           = NULL;
    int encoding                = PDF_SIMPLE_ENCODING_LATIN;
    const char *zapdb_font_name     = "ZapfDingbats";
    const char *zapdb_resource_name = "ZaDb";

    fz_var(stream);
    fz_var(font);

    fz_try(ctx) {
        // Add cheeckbox to content stream
        stream = fz_new_buffer(ctx, 1024);
        matrix = rect_to_page_space(ctx, page, position);
        fz_append_string(ctx, stream, "q\n");               // Saves the current graphics state.
        fz_append_printf(ctx, stream, "%g %g %g %g %g %g cm\n",
            matrix.a,
            matrix.b,
            matrix.c,
            matrix.d,
            matrix.e,
            matrix.f
        );                                                  // Set the transformation matrix for positioning, scaling and rotating

        // The matrix already includes scaling, so everything below this line should be drawn within the 0,0,1,1 box.
        fz_rect border_rect = {0.0, 0.0, 1.0, 1.0};
        float line_width    = 0.1;
        fz_append_string(ctx, stream, "0.0 G\n");           // Sets the gray stroke color to black
        fz_append_printf(ctx, stream, "0.1 w\n",
            line_width
        );                                                  // Set the line width
        fz_append_printf(ctx, stream, "%g %g %g %g re\n",
            border_rect.x0 + line_width / 2,
            border_rect.y0 + line_width / 2,
            border_rect.x1 - line_width / 2,
            border_rect.y1 - line_width / 2
        );                                                  // Rectangle
        fz_append_string(ctx, stream, "s\n");               // Strokes the rectangle with the current stroke color

        if (is_checked) {
            // Add Font to Resources
            font = fz_new_base14_font(ctx, zapdb_font_name);
            resources = get_or_create_dict(ctx, page->obj, PDF_NAME(Resources));
            font_dict = get_or_create_dict(ctx, resources, PDF_NAME(Font));
            font_ref  = pdf_add_simple_font(ctx, page->doc, font, encoding);
            pdf_dict_puts(ctx, font_dict, zapdb_resource_name, font_ref);

            // Add V mark to content stream
            fz_append_string(ctx, stream, "q\n");           // Saves the current graphics state.
            fz_append_string(ctx, stream, "BT\n");          // Begins a text object

            fz_point text_offset = {0.2, 0.2};
            float font_size =  (border_rect.y1 - border_rect.y0) - (line_width * 2);
            fz_append_printf(ctx, stream, "/%s %g Tf\n",
                zapdb_resource_name,
                font_size
            );                                              // Sets the font to ZaDb (ZapfDingbats)
            fz_append_printf(ctx, stream, "%g %g Td\n",
                line_width + text_offset.x,
                line_width + text_offset.y
            );                                              // Offset the text position
            fz_append_string(ctx, stream, "(4) Tj\n");      // Draw 'v' mark at the current position
            fz_append_string(ctx, stream, "ET\n");          // Ends the text object.
            fz_append_string(ctx, stream, "Q\n");           // Restores the previously saved graphics state
        }
        fz_append_string(ctx, stream, "Q\n");               // Restores the previously saved graphics state

        page_add_content_to_content_stream(ctx, page, stream);
    }
    fz_always(ctx) {
        fz_drop_buffer(ctx, stream);
        fz_drop_font(ctx, font);
    }
    fz_catch(ctx) {
        fz_rethrow(ctx);
    }
}

addCheckboxOutput add_checkbox_to_page(pdfDocument document, addCheckboxInput input) {
    addCheckboxOutput output    = { .error = NULL };
    pdf_document *pdf           = NULL;
    pdf_page *page              = NULL;
    fz_rect position = {
      input.x,
      input.y,
      input.x + input.width,
      input.y + input.height
    };

    fz_context *ctx = fz_clone_context(global_ctx);
    if (ctx == NULL) {
        output.error = strdup("Failed to clone global context");
        return output;
    }

    fz_try(ctx) {
        pdf = (pdf_document *)document.handle;
        page = pdf_load_page(ctx, pdf, input.page);

        wrap_page_contents(ctx, page);
        page_add_checkbox(ctx, page, position, input.value);

    }
    fz_always(ctx) {
        if (page != NULL) {
            fz_drop_page(ctx, &page->super);
        }
    }
    fz_catch(ctx) {
        output.error = strdup(fz_caught_message(ctx));
    }

    fz_drop_context(ctx);
    return output;
}

savePDFOutput save_pdf(pdfDocument document, const char *file_path) {
    savePDFOutput output = { .error = NULL };

    fz_context *ctx = fz_clone_context(global_ctx);
    if (ctx == NULL) {
        output.error = strdup("Failed to clone global context");
        return output;
    }

    fz_try(ctx) {
        pdf_document *doc = (pdf_document *)document.handle;

        pdf_write_options options = pdf_default_write_options;
        options.do_compress = 1;
        options.do_compress_images = 0;   // avoid recompressing image streams
        options.do_compress_fonts = 0;    // keep original font streams
        options.do_garbage = 1;           // remove dead objects only (not full rewrite)
        options.do_linear = 0;            // skip linearization (web optimization)
        options.do_incremental = 0;       // write clean file, but not in-place update
        
        pdf_save_document(ctx, doc, file_path, &options);
    }
    fz_catch(ctx) {
        output.error = strdup(fz_caught_message(ctx));
    }

    fz_drop_context(ctx);
    return output;
}

saveToPNGOutput save_to_png_file(pdfDocument document, saveToPNGInput input) {
    saveToPNGOutput output;
    output.payload = NULL;
    output.payload_length = 0;
    output.error = strdup("save_to_png_file not yet implemented");
    return output;
}

