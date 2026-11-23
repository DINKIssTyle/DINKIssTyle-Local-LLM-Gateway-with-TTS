//go:build darwin && arm64 && cgo

package mediapipeworker

/*
#include <dlfcn.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <unistd.h>

// MediaPipe Tasks C ABI for the pinned 1.1.0 distribution.
typedef struct {
    const char *buffer;
    unsigned size;
    const char *path;
    int fd, delegate, host_environment, host_system;
    const char *host_version, *ca_bundle, *app_id, *app_version;
} BaseOptions;

typedef struct {
    BaseOptions base;
    bool normalize;
    int max_input, vision_tokens, activation;
    const char *cache_dir;
} EmbedOptions;

typedef struct {
    float *values;
    uint8_t *quantized;
    uint32_t count;
    int index;
    const char *name;
} Embedding;

typedef struct {
    Embedding *vectors;
    uint32_t count;
    bool has_timestamp;
    int64_t timestamp;
} EmbedResult;

typedef struct {
    BaseOptions base;
    int max_input;
} DecisionOptions;

typedef struct {
    const char **keys, **descriptions;
    int count;
    float temperature;
    const char *instructions;
    int scoring;
    bool normalize_prior;
    void *images, *image_lengths, *audio, *audio_lengths;
} Choice;

typedef struct {
    const char *selected;
    const char **keys;
    float *probabilities;
    int count;
    float confidence;
} DecisionResult;

static void *library;
static const char *openLibrary(const char *path) {
    const char *symbols[] = {
        "MpUniversalEmbedderCreate", "MpUniversalEmbedderEmbedText",
        "MpUniversalEmbedderCloseResult", "MpUniversalEmbedderClose",
        "MpDecisionMakerCreate", "MpDecisionMakerEvaluateChoice",
        "MpDecisionMakerCloseChoiceResult", "MpDecisionMakerClose", "MpErrorFree"
    };
    library = dlopen(path, RTLD_NOW | RTLD_LOCAL);
    if (!library) return dlerror();
    for (unsigned i = 0; i < sizeof(symbols)/sizeof(symbols[0]); ++i) {
        if (!dlsym(library, symbols[i])) return symbols[i];
    }
    return NULL;
}

static int protocolOutput(void) {
    int fd = dup(STDOUT_FILENO);
    if (fd < 0) return -1;
    if (dup2(STDERR_FILENO, STDOUT_FILENO) < 0) {
        close(fd);
        return -1;
    }
    return fd;
}

static int createEmbedder(const char *path, void **out, char **err) {
    EmbedOptions options = {.base={.path=path,.fd=-1},.normalize=true,.max_input=2048};
    return ((int (*)(EmbedOptions*,void**,char**))dlsym(library,"MpUniversalEmbedderCreate"))(&options,out,err);
}
static int embed(void *encoder, const char *text, EmbedResult *out, char **err) {
    return ((int (*)(void*,const char*,EmbedResult*,char**))dlsym(library,"MpUniversalEmbedderEmbedText"))(encoder,text,out,err);
}
static void freeEmbedResult(EmbedResult *result) {
    ((void (*)(EmbedResult*))dlsym(library,"MpUniversalEmbedderCloseResult"))(result);
}
static void closeEmbedder(void *encoder) {
    ((int (*)(void*,char**))dlsym(library,"MpUniversalEmbedderClose"))(encoder,NULL);
}
static int createDecider(const char *path, void **out, char **err) {
    DecisionOptions options = {.base={.path=path,.fd=-1},.max_input=2048};
    return ((int (*)(DecisionOptions*,void**,char**))dlsym(library,"MpDecisionMakerCreate"))(&options,out,err);
}
static int choose(void *decider, const char *text, Choice *question, DecisionResult *out, char **err) {
    return ((int (*)(void*,const char*,Choice*,DecisionResult*,char**))dlsym(library,"MpDecisionMakerEvaluateChoice"))(decider,text,question,out,err);
}
static void freeDecisionResult(DecisionResult *result) {
    ((void (*)(DecisionResult*))dlsym(library,"MpDecisionMakerCloseChoiceResult"))(result);
}
static void closeDecider(void *decider) {
    ((int (*)(void*,char**))dlsym(library,"MpDecisionMakerClose"))(decider,NULL);
}
static void freeError(char *message) {
    ((void (*)(char*))dlsym(library,"MpErrorFree"))(message);
}
*/
import "C"

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"unsafe"
)

const Supported = true

func statusError(status C.int, message *C.char) error {
	detail := ""
	if message != nil {
		detail = C.GoString(message)
		C.freeError(message)
	}
	if status != 0 {
		return fmt.Errorf("MediaPipe status %d: %s", status, detail)
	}
	return nil
}

func Run(library, model string) error {
	fd := C.protocolOutput()
	if fd < 0 {
		return fmt.Errorf("cannot isolate MediaPipe protocol output")
	}
	output := os.NewFile(uintptr(fd), "mediapipe-protocol")
	defer output.Close()
	libraryPath := C.CString(library)
	defer C.free(unsafe.Pointer(libraryPath))
	if detail := C.openLibrary(libraryPath); detail != nil {
		return fmt.Errorf("cannot load MediaPipe 1.1.0: %s", C.GoString(detail))
	}
	modelPath := C.CString(model)
	defer C.free(unsafe.Pointer(modelPath))

	var encoder, decider unsafe.Pointer
	var message *C.char
	status := C.createEmbedder(modelPath, &encoder, &message)
	if err := statusError(status, message); err != nil {
		return err
	}
	defer C.closeEmbedder(encoder)
	defer func() {
		if decider != nil {
			C.closeDecider(decider)
		}
	}()
	encode := func(text string) ([]float64, error) {
		if strings.ContainsRune(text, 0) {
			return nil, fmt.Errorf("embedding text contains a null byte")
		}
		input := C.CString(text)
		defer C.free(unsafe.Pointer(input))
		var result C.EmbedResult
		var detail *C.char
		status := C.embed(encoder, input, &result, &detail)
		if err := statusError(status, detail); err != nil {
			return nil, err
		}
		defer C.freeEmbedResult(&result)
		if result.count != 1 || result.vectors == nil || result.vectors.values == nil || result.vectors.count < dimension {
			return nil, fmt.Errorf("invalid MediaPipe embedding result")
		}
		vector := make([]float64, dimension)
		for i, value := range unsafe.Slice(result.vectors.values, dimension) {
			vector[i] = float64(value)
		}
		return vector, nil
	}
	if _, err := encode("task: search result | query: memory"); err != nil {
		return err
	}
	return serve(os.Stdin, output, encode, func(req request) (reply, error) {
		if decider == nil {
			var detail *C.char
			status := C.createDecider(modelPath, &decider, &detail)
			if err := statusError(status, detail); err != nil {
				return reply{}, err
			}
		}
		return evaluateChoice(decider, req)
	})
}

func evaluateChoice(decider unsafe.Pointer, req request) (reply, error) {
	if len(req.Criteria) == 0 {
		return reply{}, fmt.Errorf("MediaPipe choice requires criteria")
	}
	keys := make([]string, 0, len(req.Criteria))
	for key := range req.Criteria {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pointers := C.calloc(C.size_t(2*len(keys)), C.size_t(unsafe.Sizeof(uintptr(0))))
	if pointers == nil {
		return reply{}, fmt.Errorf("cannot allocate MediaPipe criteria")
	}
	defer C.free(pointers)
	values := unsafe.Slice((**C.char)(pointers), 2*len(keys))
	for i, key := range keys {
		values[i] = C.CString(key)
		values[len(keys)+i] = C.CString(req.Criteria[key])
	}
	defer func() {
		for _, value := range values {
			C.free(unsafe.Pointer(value))
		}
	}()
	input := C.CString(req.Text)
	defer C.free(unsafe.Pointer(input))
	instructions := C.CString(req.Instructions)
	defer C.free(unsafe.Pointer(instructions))
	question := C.Choice{keys: &values[0], descriptions: &values[len(keys)], count: C.int(len(keys)), instructions: instructions}
	var result C.DecisionResult
	var detail *C.char
	status := C.choose(decider, input, &question, &result, &detail)
	if err := statusError(status, detail); err != nil {
		return reply{}, err
	}
	defer C.freeDecisionResult(&result)
	if result.count < 0 || int(result.count) > len(keys) || (result.count > 0 && (result.keys == nil || result.probabilities == nil)) {
		return reply{}, fmt.Errorf("invalid MediaPipe choice result")
	}
	probabilities := make(map[string]float64, int(result.count))
	scores := unsafe.Slice(result.probabilities, int(result.count))
	for i, key := range unsafe.Slice(result.keys, int(result.count)) {
		probabilities[C.GoString(key)] = float64(scores[i])
	}
	return reply{Selected: C.GoString(result.selected), Probabilities: probabilities}, nil
}
