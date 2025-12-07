import React, { useState, useEffect } from 'react'
import {
  Dialog,
  DialogActions,
  DialogContent,
  Button,
  TextField,
  Typography,
  CircularProgress,
  makeStyles,
} from '@material-ui/core'
import { useTranslate } from 'react-admin'
import { useSelector, useDispatch } from 'react-redux'
import { DialogTitle } from './DialogTitle'
import { closeAiDjDialog } from '../actions'
import { useAiDj } from '../aiDj/useAiDj'
import aiDjApi from '../aiDj/api'

const useStyles = makeStyles((theme) => ({
  prompt: {
    marginBottom: theme.spacing(2),
    fontStyle: 'italic',
    color: theme.palette.text.secondary,
  },
  textField: {
    marginBottom: theme.spacing(2),
  },
  seedInfo: {
    marginTop: theme.spacing(2),
    fontStyle: 'italic',
  },
  loadingContainer: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
  },
}))

const AUTO_FILL_PROMPTS = [
  'Chill acoustic vibes',
  'Upbeat songs from my recent plays',
  'High energy workout music',
  'Mellow favorites from the 90s',
  "Something fresh I haven't heard in a while",
  'Soft indie folk songs',
  'Energetic dance tracks',
  'Relaxing late night music',
  'Classic rock from the 80s',
  'My forgotten favorites',
]

const getRandomPrompt = () => {
  return AUTO_FILL_PROMPTS[Math.floor(Math.random() * AUTO_FILL_PROMPTS.length)]
}

export const AiDjDialog = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const dispatch = useDispatch()
  const { dialogOpen, seedTrackId, seedArtistId, loading, active, sessionId } =
    useSelector((state) => state.aiDj)
  const { startSession, endSession } = useAiDj()

  const [prompt, setPrompt] = useState('')
  const [interpreting, setInterpreting] = useState(false)

  // Set random prompt when dialog opens
  useEffect(() => {
    if (dialogOpen) {
      setPrompt(getRandomPrompt())
    }
  }, [dialogOpen])

  const handleClose = () => {
    dispatch(closeAiDjDialog())
    setPrompt('')
    setInterpreting(false)
  }

  const handleStart = async () => {
    setInterpreting(true)

    try {
      // If there's an active session, end it first
      if (active && sessionId) {
        await endSession()
      }

      // Interpret the prompt
      const preferences = await aiDjApi.interpretPrompt(prompt)

      // Build preferences object for startSession
      const prefs = {
        energy: preferences.energy || null,
        decade: preferences.decade || null,
        contexts: preferences.contexts || [],
      }

      // Check if any preferences were set
      const hasPreferences =
        prefs.energy || prefs.decade || prefs.contexts.length > 0

      // Start the session with interpreted preferences
      startSession(
        null,
        seedTrackId,
        seedArtistId,
        null,
        hasPreferences ? prefs : null,
      )
    } catch {
      // Fallback: start with no preferences
      startSession(null, seedTrackId, seedArtistId, null, null)
    } finally {
      setInterpreting(false)
    }
  }

  const isLoading = loading || interpreting

  return (
    <Dialog
      open={dialogOpen}
      onClose={handleClose}
      aria-labelledby="aidj-dialog-title"
      fullWidth
      maxWidth="sm"
    >
      <DialogTitle id="aidj-dialog-title" onClose={handleClose}>
        {translate('resources.aiDj.title', { _: 'AI DJ' })}
      </DialogTitle>
      <DialogContent>
        <Typography variant="body1" className={classes.prompt}>
          {translate('resources.aiDj.promptLabel', {
            _: 'What kind of music are you in the mood for?',
          })}
        </Typography>

        <TextField
          fullWidth
          multiline
          rows={2}
          variant="outlined"
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          placeholder={translate('resources.aiDj.promptPlaceholder', {
            _: 'e.g., "soft acoustic songs" or "upbeat 90s favorites"',
          })}
          className={classes.textField}
          disabled={isLoading}
        />

        {(seedTrackId || seedArtistId) && (
          <Typography
            variant="body2"
            color="textSecondary"
            className={classes.seedInfo}
          >
            {translate('resources.aiDj.seedInfo', {
              _: 'Using current selection as seed',
            })}
          </Typography>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose} color="primary" disabled={isLoading}>
          {translate('ra.action.cancel')}
        </Button>
        <Button
          onClick={handleStart}
          color="primary"
          variant="contained"
          disabled={isLoading || !prompt.trim()}
        >
          {isLoading ? (
            <span className={classes.loadingContainer}>
              <CircularProgress size={16} color="inherit" />
              {translate('resources.aiDj.starting', { _: 'Starting...' })}
            </span>
          ) : (
            translate('resources.aiDj.start', { _: 'Start DJ' })
          )}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
