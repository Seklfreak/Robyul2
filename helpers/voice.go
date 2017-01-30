package helpers

import "sync"

// UNASSIGNED is an alias for a guild that is not connected to VC right now
const UNASSIGNED = "___UNASSIGNED___"

var (
    // connections maps guild ids to occupier-ids
    connections = map[string]string{}

    // mutex locks connections when writing
    mutex = &sync.Mutex{}
)

// VoiceIsOccupied checks if a plugin blocks further voice connections
func VoiceIsOccupied(guild string) bool {
    return connections[guild] != UNASSIGNED
}

// VoiceOccupy marks a guild as occupied. Returns true if occupation was successful. False otherwise.
// Example usage:
// lock := helpers.VoiceOccupy(guild.ID, "music")
// helpers.RelaxAssertEqual(lock, true, nil)
func VoiceOccupy(guild string, reason string) bool {
    if VoiceIsOccupied(guild) {
        return false
    }

    mutex.Lock()
    connections[guild] = reason
    mutex.Unlock()

    return true
}

// VoiceFree marks a guild as open for new voice connections
func VoiceFree(guild string) {
    if !VoiceIsOccupied(guild) {
        return
    }

    mutex.Lock()
    connections[guild] = UNASSIGNED
    mutex.Unlock()
}
