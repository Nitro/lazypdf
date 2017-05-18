#include "faster_raster.h"

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

// Calls back into the Go code to lock a mutex
void lock_mutex(void *locks, int lock_no) {
	pthread_mutex_t *m = (pthread_mutex_t *) locks;
	if (pthread_mutex_lock(&m[lock_no]) != 0) {
		fprintf(stderr, "lock_mutex failed!");
	}
}

// Calls back into the Go code to lock a mutex
void unlock_mutex(void *locks, int lock_no) {
	pthread_mutex_t *m = (pthread_mutex_t *) locks;
	if (pthread_mutex_unlock(&m[lock_no]) != 0) {
		fprintf(stderr, "unlock_mutex failed!");
	}
}

// Initializes the lock structure in C since we can't manage
// the memory properly from Go.
fz_locks_context *new_locks() {
	fz_locks_context *locks = malloc(sizeof(fz_locks_context));
	pthread_mutex_t *mutexes =
	    malloc(sizeof(pthread_mutex_t) * FZ_LOCK_MAX);

	int i;
	for (i = 0; i < FZ_LOCK_MAX; i++) {
		pthread_mutex_init(&mutexes[i], NULL);
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
void free_locks(fz_locks_context * locks) {
	free(locks->user);
	free(locks);
}
