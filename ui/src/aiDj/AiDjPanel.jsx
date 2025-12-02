import React, { useMemo } from 'react'
import { useSelector } from 'react-redux'
import {
  Card,
  CardContent,
  Typography,
  IconButton,
  List,
  ListItem,
  ListItemText,
  ListItemAvatar,
  Avatar,
  Divider,
  makeStyles,
  Box,
} from '@material-ui/core'
import CloseIcon from '@material-ui/icons/Close'
import { useTranslate } from 'react-admin'
import { useAiDj } from './useAiDj'
import subsonic from '../subsonic'

const useStyles = makeStyles((theme) => ({
  root: {
    position: 'fixed',
    right: 16,
    bottom: 100,
    width: 300,
    maxHeight: 400,
    overflow: 'auto',
    zIndex: 1000,
    backgroundColor: theme.palette.background.paper,
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  chapterText: {
    fontStyle: 'italic',
    marginBottom: theme.spacing(1),
    marginTop: theme.spacing(1),
  },
  modeLabel: {
    textTransform: 'capitalize',
    color: theme.palette.primary.main,
  },
  trackList: {
    padding: 0,
  },
  trackAvatar: {
    width: 40,
    height: 40,
  },
  moreText: {
    textAlign: 'center',
    padding: theme.spacing(1),
  },
}))

export const AiDjPanel = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const { active, mode, chapterText, endSession } = useAiDj()

  // Get the player queue and current position
  const playerQueue = useSelector((state) => state.player?.queue || [])
  const savedPlayIndex = useSelector(
    (state) => state.player?.savedPlayIndex || 0,
  )

  // Calculate what's up next based on current position (circular queue)
  const upNextTracks = useMemo(() => {
    if (!playerQueue || playerQueue.length === 0) return []

    const currentIndex = savedPlayIndex >= 0 ? savedPlayIndex : 0
    const tracks = []

    // Start from the next track and wrap around
    for (let i = 1; i <= Math.min(5, playerQueue.length - 1); i++) {
      const nextIndex = (currentIndex + i) % playerQueue.length
      const item = playerQueue[nextIndex]
      if (item && item.song) {
        tracks.push({
          id: item.trackId,
          title: item.song.title || item.name,
          artist: item.song.artist || item.singer,
          albumId: item.song.albumId,
          updatedAt: item.song.updatedAt,
        })
      }
    }

    return tracks
  }, [playerQueue, savedPlayIndex])

  // Total remaining tracks (excluding current)
  const remainingCount = playerQueue ? Math.max(0, playerQueue.length - 1) : 0

  if (!active) return null

  const modeLabels = {
    default: translate('resources.aiDj.modes.defaultShort', { _: 'Default' }),
    nostalgia: translate('resources.aiDj.modes.nostalgiaShort', {
      _: 'Nostalgia',
    }),
    discovery: translate('resources.aiDj.modes.discoveryShort', {
      _: 'Discovery',
    }),
  }

  return (
    <Card className={classes.root} elevation={4}>
      <CardContent>
        <Box className={classes.header}>
          <Typography variant="h6">
            {translate('resources.aiDj.title', { _: 'AI DJ' })}
            {' - '}
            <span className={classes.modeLabel}>{modeLabels[mode]}</span>
          </Typography>
          <IconButton
            size="small"
            onClick={endSession}
            title={translate('resources.aiDj.end', { _: 'End DJ' })}
          >
            <CloseIcon />
          </IconButton>
        </Box>

        {chapterText && (
          <Typography variant="body2" className={classes.chapterText}>
            &ldquo;{chapterText}&rdquo;
          </Typography>
        )}

        <Divider />

        <Typography variant="subtitle2" gutterBottom style={{ marginTop: 8 }}>
          {translate('resources.aiDj.upNext', { _: 'Up Next' })}
        </Typography>

        <List dense className={classes.trackList}>
          {upNextTracks.map((track, index) => (
            <ListItem key={track.id || index} disableGutters>
              <ListItemAvatar>
                <Avatar
                  src={subsonic.getCoverArtUrl(
                    { id: track.albumId, updatedAt: track.updatedAt },
                    40,
                  )}
                  variant="rounded"
                  className={classes.trackAvatar}
                />
              </ListItemAvatar>
              <ListItemText
                primary={track.title}
                secondary={track.artist}
                primaryTypographyProps={{ noWrap: true }}
                secondaryTypographyProps={{ noWrap: true }}
              />
            </ListItem>
          ))}
        </List>

        {remainingCount > 5 && (
          <Typography
            variant="caption"
            color="textSecondary"
            className={classes.moreText}
          >
            +{remainingCount - 5}{' '}
            {translate('resources.aiDj.moreTracks', { _: 'more tracks' })}
          </Typography>
        )}
      </CardContent>
    </Card>
  )
}

export default AiDjPanel
