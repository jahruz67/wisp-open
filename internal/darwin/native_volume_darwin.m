//go:build darwin

#import <CoreAudio/CoreAudio.h>
#include <stdbool.h>
#include <stdlib.h>

typedef struct {
    AudioObjectPropertyElement channel;
    Float32 volume;
} WISVolumeChannel;

typedef struct {
    AudioDeviceID device;
    UInt32 count;
    WISVolumeChannel channels[];
} WISOutputVolume;

static AudioObjectPropertyAddress WISVolumeAddress(AudioObjectPropertyElement channel) {
    return (AudioObjectPropertyAddress){
        kAudioDevicePropertyVolumeScalar, kAudioObjectPropertyScopeOutput, channel
    };
}

bool wis_restore_output(void *state) {
    WISOutputVolume *saved = state;
    bool success = true;
    for (UInt32 i = 0; i < saved->count; i++) {
        AudioObjectPropertyAddress address = WISVolumeAddress(saved->channels[i].channel);
        if (AudioObjectSetPropertyData(saved->device, &address, 0, NULL,
                                      sizeof(Float32), &saved->channels[i].volume) != noErr) {
            success = false;
        }
    }
    free(saved);
    return success;
}

void *wis_duck_output(void) {
    AudioDeviceID device = kAudioObjectUnknown;
    UInt32 size = sizeof(device);
    AudioObjectPropertyAddress address = {
        kAudioHardwarePropertyDefaultOutputDevice,
        kAudioObjectPropertyScopeGlobal, kAudioObjectPropertyElementMain
    };
    if (AudioObjectGetPropertyData(kAudioObjectSystemObject, &address, 0, NULL,
                                  &size, &device) != noErr || device == kAudioObjectUnknown) return NULL;

    address = WISVolumeAddress(kAudioObjectPropertyElementMain);
    Boolean settable = false;
    bool master = AudioObjectIsPropertySettable(device, &address, &settable) == noErr && settable;
    UInt32 count = 1;
    if (!master) {
        address = (AudioObjectPropertyAddress){
            kAudioDevicePropertyStreamConfiguration,
            kAudioObjectPropertyScopeOutput, kAudioObjectPropertyElementMain
        };
        if (AudioObjectGetPropertyDataSize(device, &address, 0, NULL, &size) != noErr) return NULL;
        AudioBufferList *buffers = malloc(size);
        if (!buffers) return NULL;
        OSStatus status = AudioObjectGetPropertyData(device, &address, 0, NULL, &size, buffers);
        count = 0;
        if (status == noErr) {
            for (UInt32 i = 0; i < buffers->mNumberBuffers; i++) count += buffers->mBuffers[i].mNumberChannels;
        }
        free(buffers);
        if (count == 0) return NULL;
    }

    WISOutputVolume *saved = calloc(1, sizeof(WISOutputVolume) + count * sizeof(WISVolumeChannel));
    if (!saved) return NULL;
    saved->device = device;
    saved->count = count;
    for (UInt32 i = 0; i < count; i++) {
        saved->channels[i].channel = master ? kAudioObjectPropertyElementMain : i + 1;
        address = WISVolumeAddress(saved->channels[i].channel);
        size = sizeof(Float32);
        settable = false;
        if (AudioObjectIsPropertySettable(device, &address, &settable) != noErr || !settable ||
            AudioObjectGetPropertyData(device, &address, 0, NULL, &size, &saved->channels[i].volume) != noErr) {
            free(saved);
            return NULL;
        }
    }
    for (UInt32 i = 0; i < count; i++) {
        address = WISVolumeAddress(saved->channels[i].channel);
        Float32 quiet = saved->channels[i].volume * 0.01f;
        if (AudioObjectSetPropertyData(device, &address, 0, NULL, sizeof(quiet), &quiet) != noErr) {
            wis_restore_output(saved);
            return NULL;
        }
    }
    return saved;
}
