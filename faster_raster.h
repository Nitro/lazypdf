#include <fitz.h>
#include <pthread.h>

// indent -linux -br -brf

// The number of mutexes we'll allocate
#define MUTEX_COUNT 10

fz_context *cgo_fz_new_context(const fz_alloc_context * alloc,
			       const fz_locks_context * locks,
			       size_t max_store);
int cgo_ptr_cast(ptrdiff_t ptr);
void lock_mutex(void *locks, int lock_no);
void unlock_mutex(void *locks, int lock_no);
fz_locks_context *new_locks();
void free_locks(fz_locks_context * locks);
