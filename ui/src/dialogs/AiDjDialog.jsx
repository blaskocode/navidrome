import React, { useState } from 'react'
import {
  Dialog,
  DialogActions,
  DialogContent,
  Button,
  FormControl,
  FormLabel,
  RadioGroup,
  FormControlLabel,
  Radio,
  Typography,
  makeStyles,
} from '@material-ui/core'
import { useTranslate } from 'react-admin'
import { useSelector, useDispatch } from 'react-redux'
import { DialogTitle } from './DialogTitle'
import { closeAiDjDialog } from '../actions'
import { useAiDj } from '../aiDj/useAiDj'

const useStyles = makeStyles((theme) => ({
  description: {
    marginBottom: theme.spacing(2),
  },
  seedInfo: {
    marginTop: theme.spacing(2),
    fontStyle: 'italic',
  },
}))

export const AiDjDialog = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const dispatch = useDispatch()
  const { dialogOpen, seedTrackId, seedArtistId, loading } = useSelector(
    (state) => state.aiDj,
  )
  const { startSession } = useAiDj()
  const [mode, setMode] = useState('default')

  const handleClose = () => {
    dispatch(closeAiDjDialog())
  }

  const handleStart = () => {
    startSession(mode, seedTrackId, seedArtistId)
  }

  return (
    <Dialog
      open={dialogOpen}
      onClose={handleClose}
      aria-labelledby="aidj-dialog-title"
      fullWidth
      maxWidth="xs"
    >
      <DialogTitle id="aidj-dialog-title" onClose={handleClose}>
        {translate('resources.aiDj.title', { _: 'AI DJ' })}
      </DialogTitle>
      <DialogContent>
        <Typography variant="body2" className={classes.description}>
          {translate('resources.aiDj.description', {
            _: 'AI DJ will create a personalized radio from your library based on your listening history.',
          })}
        </Typography>
        <FormControl component="fieldset">
          <FormLabel component="legend">
            {translate('resources.aiDj.mode', { _: 'Mode' })}
          </FormLabel>
          <RadioGroup value={mode} onChange={(e) => setMode(e.target.value)}>
            <FormControlLabel
              value="default"
              control={<Radio />}
              label={translate('resources.aiDj.modes.default', {
                _: 'Default - Mix of favorites and discoveries',
              })}
            />
            <FormControlLabel
              value="nostalgia"
              control={<Radio />}
              label={translate('resources.aiDj.modes.nostalgia', {
                _: 'Nostalgia - Older favorites and forgotten gems',
              })}
            />
            <FormControlLabel
              value="discovery"
              control={<Radio />}
              label={translate('resources.aiDj.modes.discovery', {
                _: 'Discovery - Rarely played tracks',
              })}
            />
          </RadioGroup>
        </FormControl>
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
        <Button onClick={handleClose} color="primary">
          {translate('ra.action.cancel')}
        </Button>
        <Button
          onClick={handleStart}
          color="primary"
          variant="contained"
          disabled={loading}
        >
          {translate('resources.aiDj.start', { _: 'Start' })}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
