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
  FormGroup,
  Checkbox,
  Select,
  MenuItem,
  Typography,
  Box,
  makeStyles,
} from '@material-ui/core'
import { useTranslate } from 'react-admin'
import { useSelector, useDispatch } from 'react-redux'
import { DialogTitle } from './DialogTitle'
import { closeAiDjDialog } from '../actions'
import { useAiDj } from '../aiDj/useAiDj'

const useStyles = makeStyles((theme) => ({
  prompt: {
    marginBottom: theme.spacing(3),
    fontStyle: 'italic',
    color: theme.palette.text.secondary,
  },
  section: {
    marginBottom: theme.spacing(2),
  },
  contextRow: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
  },
  decadeSelect: {
    minWidth: 100,
  },
  seedInfo: {
    marginTop: theme.spacing(2),
    fontStyle: 'italic',
  },
}))

const ENERGY_OPTIONS = [
  { value: 'upbeat', labelKey: 'resources.aiDj.energy.upbeat', fallback: 'upbeat' },
  { value: 'soft', labelKey: 'resources.aiDj.energy.soft', fallback: 'soft' },
  { value: 'intense', labelKey: 'resources.aiDj.energy.intense', fallback: 'intense' },
  { value: '', labelKey: 'resources.aiDj.energy.any', fallback: 'any mood' },
]

const DECADE_OPTIONS = [
  { value: 1960, label: '60s' },
  { value: 1970, label: '70s' },
  { value: 1980, label: '80s' },
  { value: 1990, label: '90s' },
  { value: 2000, label: '2000s' },
  { value: 2010, label: '2010s' },
  { value: 2020, label: '2020s' },
]

export const AiDjDialog = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const dispatch = useDispatch()
  const { dialogOpen, seedTrackId, seedArtistId, loading } = useSelector(
    (state) => state.aiDj,
  )
  const { startSession } = useAiDj()

  // Preference state - resets each time dialog opens
  const [energy, setEnergy] = useState('')
  const [decadeEnabled, setDecadeEnabled] = useState(false)
  const [decade, setDecade] = useState(1990)
  const [contexts, setContexts] = useState({
    forgotten: false,
    favorites: false,
    new: false,
  })

  const handleClose = () => {
    dispatch(closeAiDjDialog())
    // Reset state when closing
    setEnergy('')
    setDecadeEnabled(false)
    setDecade(1990)
    setContexts({ forgotten: false, favorites: false, new: false })
  }

  const handleStart = () => {
    const selectedContexts = Object.entries(contexts)
      .filter(([, enabled]) => enabled)
      .map(([key]) => key)

    const preferences = {
      energy: energy || null,
      decade: decadeEnabled ? decade : null,
      contexts: selectedContexts,
    }

    // Only pass preferences if any are set
    const hasPreferences = energy || decadeEnabled || selectedContexts.length > 0
    startSession(null, seedTrackId, seedArtistId, null, hasPreferences ? preferences : null)
  }

  const toggleContext = (key) => {
    setContexts((prev) => {
      const newContexts = { ...prev, [key]: !prev[key] }
      // Mutually exclusive: favorites and new cannot both be selected
      if (key === 'favorites' && newContexts.favorites) {
        newContexts.new = false
      } else if (key === 'new' && newContexts.new) {
        newContexts.favorites = false
      }
      return newContexts
    })
  }

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
          {translate('resources.aiDj.prompt', {
            _: 'I want to hear...'
          })}
        </Typography>

        {/* Energy Selection */}
        <FormControl component="fieldset" className={classes.section}>
          <FormLabel component="legend">
            {translate('resources.aiDj.energy.label', { _: 'Energy' })}
          </FormLabel>
          <RadioGroup
            value={energy}
            onChange={(e) => setEnergy(e.target.value)}
            row
          >
            {ENERGY_OPTIONS.map((opt) => (
              <FormControlLabel
                key={opt.value || 'any'}
                value={opt.value}
                control={<Radio />}
                label={translate(opt.labelKey, { _: opt.fallback })}
              />
            ))}
          </RadioGroup>
        </FormControl>

        {/* Context Selection */}
        <FormControl component="fieldset" className={classes.section}>
          <FormLabel component="legend">
            {translate('resources.aiDj.context.label', { _: 'Context' })}
          </FormLabel>
          <FormGroup>
            {/* Decade context */}
            <Box className={classes.contextRow}>
              <FormControlLabel
                control={
                  <Checkbox
                    checked={decadeEnabled}
                    onChange={() => setDecadeEnabled(!decadeEnabled)}
                  />
                }
                label={translate('resources.aiDj.context.fromThe', {
                  _: 'from the'
                })}
              />
              <Select
                value={decade}
                onChange={(e) => setDecade(e.target.value)}
                disabled={!decadeEnabled}
                className={classes.decadeSelect}
                variant="outlined"
                size="small"
              >
                {DECADE_OPTIONS.map((opt) => (
                  <MenuItem key={opt.value} value={opt.value}>
                    {opt.label}
                  </MenuItem>
                ))}
              </Select>
            </Box>

            <FormControlLabel
              control={
                <Checkbox
                  checked={contexts.forgotten}
                  onChange={() => toggleContext('forgotten')}
                />
              }
              label={translate('resources.aiDj.context.forgotten', {
                _: "haven't heard in a while"
              })}
            />
            <FormControlLabel
              control={
                <Checkbox
                  checked={contexts.favorites}
                  onChange={() => toggleContext('favorites')}
                />
              }
              label={translate('resources.aiDj.context.favorites', {
                _: 'my favorites'
              })}
            />
            <FormControlLabel
              control={
                <Checkbox
                  checked={contexts.new}
                  onChange={() => toggleContext('new')}
                />
              }
              label={translate('resources.aiDj.context.new', {
                _: 'something new to me'
              })}
            />
          </FormGroup>
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
          {translate('resources.aiDj.start', { _: 'Start DJ' })}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
