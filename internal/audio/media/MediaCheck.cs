using System;
using Windows.Media.Control;
using System.Threading.Tasks;

class MediaCheck
{
    static async Task<int> Main()
    {
        try
        {
            var sessionManager = await GlobalSystemMediaTransportControlsSessionManager.RequestAsync();
            var session = sessionManager.GetCurrentSession();
            
            if (session == null)
            {
                return 0; // No media session
            }
            
            var playbackInfo = session.GetPlaybackInfo();
            var status = playbackInfo.PlaybackStatus;
            
            // PlaybackStatus.Playing = 4
            if (status == GlobalSystemMediaTransportControlsSessionPlaybackStatus.Playing)
            {
                return 1; // Playing
            }
            
            return 0; // Not playing (paused, stopped, etc.)
        }
        catch
        {
            return 0; // Error = assume not playing
        }
    }
}
