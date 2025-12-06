import { useCallback, useRef } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import { aiDjSpeechStart, aiDjSpeechEnd, aiDjSpeechError } from '../actions'
import aiDjApi from './api'

// Shared reference to the music player's audio instance
// This allows us to pause music synchronously when DJ starts speaking
let musicPlayerAudioInstance = null

export const setMusicPlayerAudioInstance = (instance) => {
  musicPlayerAudioInstance = instance
}

export const useDjSpeech = () => {
  const dispatch = useDispatch()
  const audioRef = useRef(null)
  const isSpeaking = useSelector((state) => state.aiDj?.isSpeaking || false)

  const playCommentary = useCallback(
    async (commentaryText, onComplete) => {
      if (!commentaryText) {
        onComplete?.()
        return
      }

      // Stop any existing speech first
      if (audioRef.current) {
        audioRef.current.pause()
        audioRef.current = null
      }

      // IMMEDIATELY pause the music player before anything else
      // This prevents music from playing over the DJ
      if (musicPlayerAudioInstance) {
        musicPlayerAudioInstance.pause()
      }

      try {
        // Dispatch speech start to update Redux state and disable controls
        dispatch(aiDjSpeechStart())

        // Fetch audio from backend with the commentary text
        const audioBlob = await aiDjApi.getCommentaryAudio(commentaryText)
        const audioUrl = URL.createObjectURL(audioBlob)

        // Create and play audio element
        const audio = new Audio(audioUrl)
        audioRef.current = audio

        audio.onended = () => {
          dispatch(aiDjSpeechEnd())
          URL.revokeObjectURL(audioUrl)
          audioRef.current = null
          onComplete?.()
        }

        audio.onerror = () => {
          dispatch(aiDjSpeechError('Audio playback failed'))
          URL.revokeObjectURL(audioUrl)
          audioRef.current = null
          onComplete?.()
        }

        await audio.play()
      } catch (error) {
        dispatch(aiDjSpeechError(error.message))
        onComplete?.()
      }
    },
    [dispatch],
  )

  const stopSpeech = useCallback(() => {
    if (audioRef.current) {
      audioRef.current.pause()
      audioRef.current = null
      dispatch(aiDjSpeechEnd())
    }
  }, [dispatch])

  return {
    isSpeaking,
    playCommentary,
    stopSpeech,
  }
}

export default useDjSpeech
