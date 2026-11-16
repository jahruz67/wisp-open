//go:build darwin

#import <CoreAudio/CoreAudio.h>
#include <stdbool.h>
#include <stdlib.h>

typedef struct {
    AudioObjectPropertyElement channel;
    UInt32 muted;
} WISMuteChannel;

typedef struct {
    AudioDeviceID device;
    UInt32 count;
    WISMuteChannel channels[];
} WISOutputMute;

static AudioObjectPropertyAddress WISMuteAddress(AudioObjectPropertyElement channel) {
    return (AudioObjectPropertyAddress){
        kAudioDevicePropertyMute, kAudioObjectPropertyScopeOutput, channel
    };
}

bool wis_restore_output_mute(void *state) {
    WISOutputMute *saved = state;
    bool success = true;
    for (UInt32 i = 0; i < saved->count; i++) {
        AudioObjectPropertyAddress address = WISMuteAddress(saved->channels[i].channel);
        if (AudioObjectSetPropertyData(saved->device, &address, 0, NULL,
                                      sizeof(UInt32), &saved->channels[i].muted) != noErr) {
            success = false;
        }
    }
    free(saved);
    return success;
}

void *wis_mute_output(void) {
    AudioDeviceID device = kAudioObjectUnknown;
    UInt32 size = sizeof(device);
    AudioObjectPropertyAddress address = {
        kAudioHardwarePropertyDefaultOutputDevice,
        kAudioObjectPropertyScopeGlobal, kAudioObjectPropertyElementMain
    };
    if (AudioObjectGetPropertyData(kAudioObjectSystemObject, &address, 0, NULL,
                                  &size, &device) != noErr || device == kAudioObjectUnknown) return NULL;

    address = WISMuteAddress(kAudioObjectPropertyElementMain);
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

    WISOutputMute *saved = calloc(1, sizeof(WISOutputMute) + count * sizeof(WISMuteChannel));
    if (!saved) return NULL;
    saved->device = device;
    saved->count = count;
    for (UInt32 i = 0; i < count; i++) {
        saved->channels[i].channel = master ? kAudioObjectPropertyElementMain : i + 1;
        address = WISMuteAddress(saved->channels[i].channel);
        size = sizeof(UInt32);
        settable = false;
        if (AudioObjectIsPropertySettable(device, &address, &settable) != noErr || !settable ||
            AudioObjectGetPropertyData(device, &address, 0, NULL, &size, &saved->channels[i].muted) != noErr) {
            free(saved);
            return NULL;
        }
    }
    for (UInt32 i = 0; i < count; i++) {
        address = WISMuteAddress(saved->channels[i].channel);
        UInt32 muted = 1;
        if (AudioObjectSetPropertyData(device, &address, 0, NULL, sizeof(muted), &muted) != noErr) {
            wis_restore_output_mute(saved);
            return NULL;
        }
    }
    return saved;
}
