#include "faster_raster.h"

#include <stdio.h>
#include <string.h>

// Format with:
// indent -linux -br -brf

// Have to wrap this macro so we can call from Cgo
fz_context *cgo_fz_new_context(const fz_alloc_context * alloc,
			       const fz_locks_context * locks,
			       size_t max_store) {
	return fz_new_context(alloc, locks, max_store);
}

// Cast a ptrdiff_t to an int for use in Cgo. Go types won't let
// us do it in Go.
int cgo_ptr_cast(ptrdiff_t ptr) {
	return (int)(ptr);
}

// Wrap fz_open_document, which uses a try/catch exception handler
// that we can't easily use from Go.
fz_document *cgo_open_document(fz_context * ctx, const char *filename,
			       const char *default_ext) {
	fz_document *doc = NULL;
	int failed = 0;

	fz_try(ctx) {
		doc = fz_open_document(ctx, filename);
	}
	fz_catch(ctx) {
		fprintf(stderr, "Trying with default file extension for '%s'",
			filename);
		failed = 1;
	}

	if (failed) {
		fz_try(ctx) {
			doc =
			    open_document_with_extension(ctx, filename,
							 default_ext);
		}
		fz_catch(ctx) {
			fprintf(stderr, "cannot open document '%s': %s\n",
				filename, fz_caught_message(ctx));
			return NULL;
		}
	}

	return doc;
}

fz_document *open_document_with_extension(fz_context * ctx,
					  const char *filename,
					  const char *default_ext) {
	const fz_document_handler *handler;
	fz_stream *file;
	fz_document *doc = NULL;

	handler = fz_recognize_document(ctx, default_ext);
	if (!handler)
		fz_throw(ctx, FZ_ERROR_GENERIC,
			 "cannot find doc handler for file extension: %s for document '%s'",
			 default_ext, filename);

	if (handler->open)
		return handler->open(ctx, filename);

	file = fz_open_file(ctx, filename);

	fz_try(ctx)
	    doc = handler->open_with_stream(ctx, file);
	fz_always(ctx)
	    fz_drop_stream(ctx, file);
	fz_catch(ctx)
	    fz_rethrow(ctx);

	return doc;
}

// Wrap fz_drop_document to handle the exception trap when something is
// wrong. We can't easily do this from Go.
void cgo_drop_document(fz_context * ctx, fz_document * doc) {
	fz_try(ctx) {
		fz_drop_document(ctx, doc);
	}
	fz_catch(ctx) {
		fprintf(stderr, "cannot drop document: %s\n",
			fz_caught_message(ctx));
	}
}

// Calls back into the Go code to lock a mutex
void lock_mutex(void *locks, int lock_no) {
	pthread_mutex_t *m = (pthread_mutex_t *) locks;
	int result;

	if ((result = pthread_mutex_lock(&m[lock_no])) != 0) {
		fprintf(stderr, "lock_mutex failed! %s\n", strerror(result));
	}
}

// Calls back into the Go code to lock a mutex
void unlock_mutex(void *locks, int lock_no) {
	pthread_mutex_t *m = (pthread_mutex_t *) locks;
	int result;

	if ((result = pthread_mutex_unlock(&m[lock_no])) != 0) {
		fprintf(stderr, "unlock_mutex failed! %s\n", strerror(result));
	}
}

// Initializes the lock structure in C since we can't manage
// the memory properly from Go.
fz_locks_context *new_locks() {
	fz_locks_context *locks = malloc(sizeof(fz_locks_context));

	if (locks == NULL) {
		fprintf(stderr, "Unable to allocate locks!\n");
		return NULL;
	}

	pthread_mutex_t *mutexes =
	    malloc(sizeof(pthread_mutex_t) * FZ_LOCK_MAX);

	if (mutexes == NULL) {
		fprintf(stderr, "Unable to allocate mutexes!\n");
		return NULL;
	}

	int i, result;
	for (i = 0; i < FZ_LOCK_MAX; i++) {
		result = pthread_mutex_init(&mutexes[i], NULL);
		if (result != 0) {
			fprintf(stderr, "Failed to initialize mutex: %s\n",
				strerror(result));
		}
	}

	// Pass in the initialized mutexes and the two C funcs
	// that will handle the pthread mutexes.
	locks->lock = &lock_mutex;
	locks->unlock = &unlock_mutex;
	locks->user = mutexes;

	return locks;
}

// Free the lock structure in C since we allocated the memory
// here.
void free_locks(fz_locks_context ** locks) {
	free((*locks)->user);
	free(*locks);
	*locks = NULL;
}

// Read a property from the PDF object by key name
static pdf_obj *pdf_lookup_inherited_page_item(fz_context * ctx, pdf_obj * node,
					       pdf_obj * key) {
	pdf_obj *node2 = node;
	pdf_obj *val;

	fz_try(ctx) {
		do {
			val = pdf_dict_get(ctx, node, key);
			if (val)
				break;
			if (pdf_mark_obj(ctx, node))
				fz_throw(ctx, FZ_ERROR_GENERIC,
					 "cycle in page tree (parents)");
			node = pdf_dict_get(ctx, node, PDF_NAME(Parent));
		}
		while (node);
	}
	fz_always(ctx) {
		do {
			pdf_unmark_obj(ctx, node2);
			if (node2 == node)
				break;
			node2 = pdf_dict_get(ctx, node2, PDF_NAME(Parent));
		}
		while (node2);
	}
	fz_catch(ctx) {
		fz_rethrow(ctx);
	}

	return val;
}

// Return an integer representing the rotation of a page in degrees
int get_rotation(fz_context * ctx, fz_page * page) {
	// We know we have a pdf_page here in 'page' so we cast it to a pdf_page *
	pdf_obj *page_obj = ((pdf_page *) page)->obj;
	return pdf_to_int(ctx,
			  pdf_lookup_inherited_page_item(ctx, page_obj,
							 PDF_NAME(Rotate)));
}

// Wrapper for fz_load_page which returns NULL when an invalid page number
// is requested. fz_load_page can throw for various reasons and we don't
// want the whole process to go down when it does.
fz_page *load_page(fz_context * ctx, fz_document * doc, int number) {
	fz_page *page;

	fz_try(ctx) {
		page = fz_load_page(ctx, doc, number);
	}
	fz_catch(ctx) {
		fprintf(stderr, "Invalid page number requested: %d\n", number);
		page = NULL;
	}

	return page;
}
