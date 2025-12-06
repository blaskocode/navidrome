import { useCallback, useRef } from 'react'
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
import { useDjSpeech } from './useDjSpeech'

export const useAiDj = () => {
  const dispatch = useDispatch()
  const aiDj = useSelector((state) => state.aiDj || {})
  const playerMode = useSelector((state) => state.player?.mode || 'order')
  const playerQueue = useSelector((state) => state.player?.queue || [])
  const { isSpeaking, playCommentary } = useDjSpeech()

  // Track the last known chapterText to detect set transitions
  const lastChapterTextRef = useRef(null)

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

        // Store initial chapter text
        lastChapterTextRef.current = state.chapterText

        // Auto-play first track if available
        // Only add tracks from the CURRENT set, not future sets
        // This prevents auto-advance into the next set before DJ speaks
        const currentSetTracks = state.sets
          ? state.sets.find((s) => s.isCurrent)?.tracks || []
          : state.upNext || []

        if (currentSetTracks.length > 0) {
          const tracksById = {}
          currentSetTracks.forEach((track) => {
            tracksById[track.id] = track
          })

          // Save current play mode and set to 'order' so player stops at end of set
          // (not 'orderLoop' which would loop back before DJ can speak)
          dispatch(aiDjSavePlayMode(playerMode))
          dispatch(setPlayMode('order'))

          // Play commentary first, then start music
          playCommentary(state.chapterText, () => {
            dispatch(playTracks(tracksById, Object.keys(tracksById)))
          })
        }

        return state
      } catch (error) {
        dispatch(
          startSessionFailure(error.message || 'Failed to start AI DJ session'),
        )
        throw error
      }
    },
    [dispatch, playerMode, playCommentary],
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

        // Check if we transitioned to a new set (chapterText changed)
        const isNewSet =
          state.chapterText && state.chapterText !== lastChapterTextRef.current
        lastChapterTextRef.current = state.chapterText

        // Only add tracks from the CURRENT set to prevent auto-advance
        const currentSetTracks = state.sets
          ? state.sets.find((s) => s.isCurrent)?.tracks || []
          : state.upNext || []

        if (currentSetTracks.length > 0) {
          // Get current player track IDs
          const currentTrackIds = new Set(
            playerQueue.map((item) => item.trackId),
          )

          // Find tracks from current set that aren't in the player queue
          const newTracks = currentSetTracks.filter(
            (track) => !currentTrackIds.has(track.id),
          )

          if (newTracks.length > 0) {
            const newTracksById = {}
            newTracks.forEach((track) => {
              newTracksById[track.id] = track
            })

            // If new set, play commentary then START playing new tracks (replace queue)
            // If same set, just add tracks to existing queue
            if (isNewSet) {
              playCommentary(state.chapterText, () => {
                dispatch(playTracks(newTracksById, Object.keys(newTracksById)))
              })
            } else {
              dispatch(addTracks(newTracksById, Object.keys(newTracksById)))
            }
          }
        }

        return state
      } catch (error) {
        dispatch(nextTrackFailure(error.message || 'Failed to get next track'))
        throw error
      }
    },
    [dispatch, aiDj.sessionId, playerQueue, playCommentary],
  )

  const skipTrack = useCallback(
    async (skippedTrackId) => {
      if (!aiDj.sessionId) return

      dispatch(skipTrackAction(aiDj.sessionId, skippedTrackId))
      try {
        const state = await aiDjApi.skip(aiDj.sessionId, skippedTrackId)
        dispatch(skipTrackSuccess(state))

        // Check if we pivoted to a new theme (chapterText changed)
        const isPivot =
          state.chapterText && state.chapterText !== lastChapterTextRef.current
        lastChapterTextRef.current = state.chapterText

        // Only add tracks from the CURRENT set to prevent auto-advance
        const currentSetTracks = state.sets
          ? state.sets.find((s) => s.isCurrent)?.tracks || []
          : state.upNext || []

        if (currentSetTracks.length > 0) {
          // Get current player track IDs
          const currentTrackIds = new Set(
            playerQueue.map((item) => item.trackId),
          )

          // Find tracks from current set that aren't in the player queue
          const newTracks = currentSetTracks.filter(
            (track) => !currentTrackIds.has(track.id),
          )

          if (newTracks.length > 0) {
            const newTracksById = {}
            newTracks.forEach((track) => {
              newTracksById[track.id] = track
            })

            // If pivot, play commentary then START playing new tracks (replace queue)
            // If same set, just add tracks to existing queue
            if (isPivot) {
              playCommentary(state.chapterText, () => {
                dispatch(playTracks(newTracksById, Object.keys(newTracksById)))
              })
            } else {
              dispatch(addTracks(newTracksById, Object.keys(newTracksById)))
            }
          }
        }

        return state
      } catch (error) {
        dispatch(skipTrackFailure(error.message || 'Failed to skip track'))
        throw error
      }
    },
    [dispatch, aiDj.sessionId, playerQueue, playCommentary],
  )

  const clearError = useCallback(() => {
    dispatch(clearAiDjError())
  }, [dispatch])

  return {
    ...aiDj,
    isSpeaking,
    startSession,
    endSession,
    nextTrack,
    skipTrack,
    clearError,
  }
}

export default useAiDj
