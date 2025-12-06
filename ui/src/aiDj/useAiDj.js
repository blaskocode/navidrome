import { useCallback } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import {
  startSession as startSessionAction,
  startSessionSuccess,
  startSessionFailure,
  endSession as endSessionAction,
  endSessionSuccess,
  endSessionFailure,
  nextTrack as nextTrackAction,
  nextTrackSuccess,
  nextTrackFailure,
  skipTrack as skipTrackAction,
  skipTrackSuccess,
  skipTrackFailure,
  clearAiDjError,
  aiDjSavePlayMode,
} from '../actions'
import { playTracks, setPlayMode, addTracks } from '../actions/player'
import aiDjApi from './api'

export const useAiDj = () => {
  const dispatch = useDispatch()
  const aiDj = useSelector((state) => state.aiDj || {})
  const playerMode = useSelector((state) => state.player?.mode || 'order')
  const playerQueue = useSelector((state) => state.player?.queue || [])

  const startSession = useCallback(
    async (
      mode = null,
      seedTrackId,
      seedArtistId,
      seedPlaylistId,
      preferences,
    ) => {
      dispatch(
        startSessionAction(mode, seedTrackId, seedArtistId, seedPlaylistId),
      )
      try {
        const state = await aiDjApi.startSession(
          mode,
          seedTrackId,
          seedArtistId,
          seedPlaylistId,
          preferences,
        )
        dispatch(startSessionSuccess(state))

        // Auto-play first track if available
        if (state.upNext && state.upNext.length > 0) {
          const tracksById = {}
          state.upNext.forEach((track) => {
            tracksById[track.id] = track
          })

          // Save current play mode and set to orderLoop for continuous playback
          dispatch(aiDjSavePlayMode(playerMode))
          dispatch(setPlayMode('orderLoop'))

          dispatch(playTracks(tracksById, Object.keys(tracksById)))
        }

        return state
      } catch (error) {
        dispatch(
          startSessionFailure(error.message || 'Failed to start AI DJ session'),
        )
        throw error
      }
    },
    [dispatch, playerMode],
  )

  const endSession = useCallback(async () => {
    if (!aiDj.sessionId) return

    dispatch(endSessionAction(aiDj.sessionId))
    try {
      await aiDjApi.endSession(aiDj.sessionId)

      // Restore previous play mode if saved
      if (
        aiDj.previousPlayMode !== null &&
        aiDj.previousPlayMode !== undefined
      ) {
        dispatch(setPlayMode(aiDj.previousPlayMode))
      }

      dispatch(endSessionSuccess())
    } catch (error) {
      dispatch(
        endSessionFailure(error.message || 'Failed to end AI DJ session'),
      )
      throw error
    }
  }, [dispatch, aiDj.sessionId, aiDj.previousPlayMode])

  const nextTrack = useCallback(
    async (playedTrackId) => {
      if (!aiDj.sessionId) return

      dispatch(nextTrackAction(aiDj.sessionId, playedTrackId))
      try {
        const state = await aiDjApi.next(aiDj.sessionId, playedTrackId)
        dispatch(nextTrackSuccess(state))

        // Sync player queue with backend state
        // The backend returns the full queue of remaining tracks across all sets
        if (state.upNext && state.upNext.length > 0) {
          // Get current player track IDs
          const currentTrackIds = new Set(
            playerQueue.map((item) => item.trackId),
          )

          // Check if there are any new tracks we don't have
          const hasNewTracks = state.upNext.some(
            (track) => !currentTrackIds.has(track.id),
          )

          // If there are new tracks, add them to keep the queue growing
          if (hasNewTracks) {
            const newTracks = state.upNext.filter(
              (track) => !currentTrackIds.has(track.id),
            )
            const newTracksById = {}
            newTracks.forEach((track) => {
              newTracksById[track.id] = track
            })
            dispatch(addTracks(newTracksById, Object.keys(newTracksById)))
          }
        }

        return state
      } catch (error) {
        dispatch(nextTrackFailure(error.message || 'Failed to get next track'))
        throw error
      }
    },
    [dispatch, aiDj.sessionId, playerQueue],
  )

  const skipTrack = useCallback(
    async (skippedTrackId) => {
      if (!aiDj.sessionId) return

      dispatch(skipTrackAction(aiDj.sessionId, skippedTrackId))
      try {
        const state = await aiDjApi.skip(aiDj.sessionId, skippedTrackId)
        dispatch(skipTrackSuccess(state))

        // Add any new tracks to the player queue that aren't already there
        if (state.upNext && state.upNext.length > 0) {
          // Get current player track IDs
          const currentTrackIds = new Set(
            playerQueue.map((item) => item.trackId),
          )

          // Find tracks that are new (not in current player queue)
          const newTracks = state.upNext.filter(
            (track) => !currentTrackIds.has(track.id),
          )

          if (newTracks.length > 0) {
            const newTracksById = {}
            newTracks.forEach((track) => {
              newTracksById[track.id] = track
            })
            dispatch(addTracks(newTracksById, Object.keys(newTracksById)))
          }
        }

        return state
      } catch (error) {
        dispatch(skipTrackFailure(error.message || 'Failed to skip track'))
        throw error
      }
    },
    [dispatch, aiDj.sessionId, playerQueue],
  )

  const clearError = useCallback(() => {
    dispatch(clearAiDjError())
  }, [dispatch])

  return {
    ...aiDj,
    startSession,
    endSession,
    nextTrack,
    skipTrack,
    clearError,
  }
}

export default useAiDj
