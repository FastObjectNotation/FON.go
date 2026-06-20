#ifndef FON_H
#define FON_H

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include <stddef.h>


/* ==================== RESULT CODES ==================== */

#define FON_OK                   0
#define FON_ERROR_FILE_NOT_FOUND 1
#define FON_ERROR_PARSE_FAILED   2
#define FON_ERROR_WRITE_FAILED   3
#define FON_ERROR_INVALID_ARGUMENT 4


/* ==================== ERROR STRUCT ==================== */

/*
 * On error, the native functions write a result code and a UTF-8 message
 * (null-terminated, max 255 bytes) into a caller-supplied FonError.
 * Pass NULL if you do not need the error details.
 */
typedef struct {
    int32_t code;
    char    message[256];
} FonError;


/* ==================== VERSION ==================== */

/* Returns a static, null-terminated version string (e.g. "0.2.1"). */
const char* fon_version(void);


/* ==================== CONFIGURATION ==================== */

/* Enable (enable != 0) or disable raw-blob unpacking during deserialization. */
void fon_set_raw_unpack(int32_t enable);

/* Set the maximum nesting depth for deserialization (clamped to >= 1). */
void fon_set_max_depth(int32_t depth);


/* ==================== MEMORY MANAGEMENT ==================== */

/*
 * All opaque handles are void*.
 * fon_dump_free  — free a Dump you own.
 * fon_collection_free — free a Collection you own.
 *
 * OWNERSHIP RULES
 * ---------------
 * fon_dump_create / fon_collection_create   → caller owns the returned handle.
 * fon_dump_add                              → transfers Collection ownership to the Dump;
 *                                             caller MUST NOT free or use the Collection handle.
 * fon_collection_add_collection             → same ownership transfer to the parent.
 * fon_collection_add_collection_array       → transfers every child handle to the parent.
 * fon_dump_get / fon_collection_get_collection / fon_collection_get_collection_array
 *   → returns borrowed handles; caller MUST NOT free them.
 * fon_deserialize_dump_from_buffer / fon_deserialize_from_file
 *   → caller owns the returned Dump handle; free via fon_dump_free.
 * fon_deserialize_collection_from_buffer    → caller owns the returned Collection handle.
 */

void* fon_dump_create(void);
void  fon_dump_free(void* dump);
int64_t fon_dump_size(void* dump);

/*
 * Returns a BORROWED pointer to the collection at index.
 * Valid while the dump is alive. Do NOT free.
 */
void* fon_dump_get(void* dump, uint64_t index);

void* fon_collection_create(void);
void  fon_collection_free(void* collection);
int64_t fon_collection_size(void* collection);


/* ==================== SERIALIZATION ==================== */

int32_t fon_serialize_to_file(
    void*       dump,
    const char* path,
    int32_t     max_threads,
    FonError*   error
);

/*
 * Two-call serialization pattern:
 *   1. Call with buffer=NULL, buffer_size=0 → *required_size receives byte count.
 *   2. Allocate buffer[required_size], call again to fill it.
 * Output is NOT null-terminated; required_size is the exact UTF-8 byte count.
 */
int32_t fon_serialize_dump_to_buffer(
    void*     dump,
    uint8_t*  buffer,
    int64_t   buffer_size,
    int64_t*  required_size,
    int32_t   max_threads,
    FonError* error
);

int32_t fon_serialize_collection_to_buffer(
    void*     collection,
    uint8_t*  buffer,
    int64_t   buffer_size,
    int64_t*  required_size,
    FonError* error
);


/* ==================== DESERIALIZATION ==================== */

/* Returns an owned Dump handle; free via fon_dump_free. NULL on error. */
void* fon_deserialize_from_file(
    const char* path,
    int32_t     max_threads,
    FonError*   error
);

/* data need not be null-terminated. Returns an owned Dump handle. NULL on error. */
void* fon_deserialize_dump_from_buffer(
    const uint8_t* data,
    int64_t        size,
    int32_t        max_threads,
    FonError*      error
);

/* Parses one line/record. Returns an owned Collection handle. NULL on error. */
void* fon_deserialize_collection_from_buffer(
    const uint8_t* data,
    int64_t        size,
    FonError*      error
);


/* ==================== DUMP ADD OPERATIONS ==================== */

/*
 * Transfers ownership of 'collection' to 'dump'. After this call the caller
 * MUST NOT free or use the collection handle.
 */
int32_t fon_dump_add(
    void*     dump,
    uint64_t  id,
    void*     collection,
    FonError* error
);


/* ==================== COLLECTION ADD OPERATIONS ==================== */

int32_t fon_collection_add_int(
    void*       collection,
    const char* key,
    int32_t     value,
    FonError*   error
);

int32_t fon_collection_add_long(
    void*       collection,
    const char* key,
    int64_t     value,
    FonError*   error
);

int32_t fon_collection_add_float(
    void*       collection,
    const char* key,
    float       value,
    FonError*   error
);

int32_t fon_collection_add_double(
    void*       collection,
    const char* key,
    double      value,
    FonError*   error
);

/* value: 0 = false, non-zero = true */
int32_t fon_collection_add_bool(
    void*       collection,
    const char* key,
    int32_t     value,
    FonError*   error
);

int32_t fon_collection_add_string(
    void*       collection,
    const char* key,
    const char* value,
    FonError*   error
);

int32_t fon_collection_add_int_array(
    void*          collection,
    const char*    key,
    const int32_t* values,
    int64_t        count,
    FonError*      error
);

int32_t fon_collection_add_float_array(
    void*         collection,
    const char*   key,
    const float*  values,
    int64_t       count,
    FonError*     error
);

/*
 * Transfers ownership of child to parent. After this call the caller
 * MUST NOT free or use the child handle.
 */
int32_t fon_collection_add_collection(
    void*       parent,
    const char* key,
    void*       child,
    FonError*   error
);

/*
 * Transfers ownership of every handle in children[] to parent.
 * After this call the caller MUST NOT free or use any child handle.
 */
int32_t fon_collection_add_collection_array(
    void*        parent,
    const char*  key,
    void**       children,
    int64_t      count,
    FonError*    error
);


/* ==================== COLLECTION GET OPERATIONS ==================== */

int32_t fon_collection_get_int(
    void*       collection,
    const char* key,
    int32_t*    value,
    FonError*   error
);

int32_t fon_collection_get_long(
    void*       collection,
    const char* key,
    int64_t*    value,
    FonError*   error
);

int32_t fon_collection_get_float(
    void*       collection,
    const char* key,
    float*      value,
    FonError*   error
);

int32_t fon_collection_get_double(
    void*       collection,
    const char* key,
    double*     value,
    FonError*   error
);

/* Writes 0 or 1 into *value. */
int32_t fon_collection_get_bool(
    void*       collection,
    const char* key,
    int32_t*    value,
    FonError*   error
);

/*
 * Copies the string value (null-terminated) into buffer[0..buffer_size-1].
 * buffer_size must be > 0.
 */
int32_t fon_collection_get_string(
    void*       collection,
    const char* key,
    uint8_t*    buffer,
    int64_t     buffer_size,
    FonError*   error
);

/*
 * Two-call pattern: pass buffer=NULL, buffer_size=0 to get *actual_size,
 * then allocate and call again to fill.
 */
int32_t fon_collection_get_int_array(
    void*       collection,
    const char* key,
    int32_t*    buffer,
    int64_t     buffer_size,
    int64_t*    actual_size,
    FonError*   error
);

int32_t fon_collection_get_float_array(
    void*       collection,
    const char* key,
    float*      buffer,
    int64_t     buffer_size,
    int64_t*    actual_size,
    FonError*   error
);

/*
 * Returns a BORROWED pointer to the nested collection.
 * Caller MUST NOT free it. NULL if key is missing or not a nested collection.
 */
void* fon_collection_get_collection(
    void*       parent,
    const char* key,
    FonError*   error
);

/*
 * Two-call pattern for array of borrowed collection pointers.
 * Caller MUST NOT free any returned handle.
 */
int32_t fon_collection_get_collection_array(
    void*     parent,
    const char* key,
    void**    buffer,
    int64_t   buffer_size,
    int64_t*  actual_size,
    FonError* error
);


#ifdef __cplusplus
}
#endif

#endif /* FON_H */
