# Check if media is playing using Windows SMTC
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | ? { $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation`1' })[0]

function Await($WinRtTask, $ResultType) {
    $asTask = $asTaskGeneric.MakeGenericMethod($ResultType)
    $netTask = $asTask.Invoke($null, @($WinRtTask))
    $netTask.Wait(-1) | Out-Null
    $netTask.Result
}

try {
    [Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager,Windows.Media,ContentType=WindowsRuntime] | Out-Null
    $sessionManager = Await ([Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager]::RequestAsync()) ([Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager])
    
    $session = $sessionManager.GetCurrentSession()
    if ($null -eq $session) {
        exit 0  # No session = not playing
    }
    
    $playbackInfo = $session.GetPlaybackInfo()
    $status = $playbackInfo.PlaybackStatus
    
    # 4 = Playing
    if ($status -eq 4) {
        exit 1  # Playing
    }
    
    exit 0  # Not playing
}
catch {
    exit 0  # Error = not playing
}
