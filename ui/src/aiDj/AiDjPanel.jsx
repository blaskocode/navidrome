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
    width: 320,
    maxHeight: 500,
    overflow: 'auto',
    zIndex: 1000,
    backgroundColor: theme.palette.background.paper,
    borderRadius: theme.shape.borderRadius,
    boxShadow: theme.shadows[8],
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
  setContainer: {
    marginBottom: theme.spacing(1),
  },
  setHeader: {
    display: 'flex',
    alignItems: 'center',
    marginBottom: theme.spacing(0.5),
  },
  setTitle: {
    fontWeight: 600,
    color: theme.palette.primary.main,
  },
  setCommentary: {
    fontStyle: 'italic',
    fontSize: '0.85rem',
    color: theme.palette.text.secondary,
    marginBottom: theme.spacing(1),
    paddingLeft: theme.spacing(1),
    borderLeft: `2px solid ${theme.palette.primary.light}`,
  },
  setDivider: {
    marginTop: theme.spacing(1.5),
    marginBottom: theme.spacing(1.5),
  },
}))

export const AiDjPanel = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const { active, mode, chapterText, endSession } = useAiDj()

  // Get sets from AI DJ state
  const sets = useSelector((state) => state.aiDj?.sets || [])

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

        {/* Display sets with headers if available, otherwise show legacy chapterText */}
        {sets.length > 0 ? (
          <>
            <Divider />
            <Typography
              variant="subtitle2"
              gutterBottom
              style={{ marginTop: 8 }}
            >
              {translate('resources.aiDj.upNext', { _: 'Up Next' })}
            </Typography>

            {sets.map((set, setIndex) => (
              <Box
                key={set.themeId + setIndex}
                className={classes.setContainer}
              >
                {/* Set header */}
                <Box className={classes.setHeader}>
                  <Typography variant="subtitle2" className={classes.setTitle}>
                    {set.isCurrent ? '▶ ' : ''}
                    {set.themeName}
                  </Typography>
                </Box>

                {/* Set commentary */}
                {set.commentary && (
                  <Typography variant="body2" className={classes.setCommentary}>
                    &ldquo;{set.commentary}&rdquo;
                  </Typography>
                )}

                {/* Set tracks */}
                <List dense className={classes.trackList}>
                  {set.tracks?.map((track, trackIndex) => (
                    <ListItem key={track.id || trackIndex} disableGutters>
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

                {/* Divider between sets (except last) */}
                {setIndex < sets.length - 1 && (
                  <Divider className={classes.setDivider} />
                )}
              </Box>
            ))}
          </>
        ) : (
          <>
            {chapterText && (
              <Typography variant="body2" className={classes.chapterText}>
                &ldquo;{chapterText}&rdquo;
              </Typography>
            )}

            <Divider />

            <Typography
              variant="subtitle2"
              gutterBottom
              style={{ marginTop: 8 }}
            >
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
          </>
        )}
      </CardContent>
    </Card>
  )
}

export default AiDjPanel
