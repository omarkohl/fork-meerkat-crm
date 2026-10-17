import { useState, useEffect, useCallback, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Divider,
  IconButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import EventIcon from '@mui/icons-material/Event';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import { buildIcsFeedUrl, createIcsFeed, getIcsFeed, IcsFeedStatus, revokeIcsFeed } from '../api/icsFeed';
import { useSnackbar } from '../context/SnackbarContext';

export default function IcsFeedSettings() {
  const { t } = useTranslation();
  const { showSuccess, showError } = useSnackbar();

  const [status, setStatus] = useState<IcsFeedStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  // The URL is only known right after creating or regenerating.
  const [feedUrl, setFeedUrl] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const copiedTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(
    () => () => {
      clearTimeout(copiedTimer.current);
    },
    [],
  );

  const [confirm, setConfirm] = useState<'regenerate' | 'revoke' | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      setStatus(await getIcsFeed());
    } catch (err) {
      setError(err instanceof Error ? err.message : t('icsFeed.loadError'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const handleGenerate = async () => {
    setBusy(true);
    try {
      const result = await createIcsFeed();
      setConfirm(null);
      setStatus({
        enabled: result.enabled,
        created_at: result.created_at,
        last_fetched_at: result.last_fetched_at,
      });
      setFeedUrl(buildIcsFeedUrl(result.token));
      setCopied(false);
    } catch (err) {
      showError(err instanceof Error ? err.message : t('icsFeed.createError'));
    } finally {
      setBusy(false);
    }
  };

  const handleRevoke = async () => {
    setBusy(true);
    try {
      await revokeIcsFeed();
      setConfirm(null);
      setStatus({ enabled: false, created_at: null, last_fetched_at: null });
      showSuccess(t('icsFeed.revokeSuccess'));
    } catch (err) {
      showError(err instanceof Error ? err.message : t('icsFeed.revokeError'));
    } finally {
      setBusy(false);
    }
  };

  const handleCopy = async () => {
    if (!feedUrl) return;
    try {
      await navigator.clipboard.writeText(feedUrl);
      setCopied(true);
      clearTimeout(copiedTimer.current);
      copiedTimer.current = setTimeout(() => {
        setCopied(false);
      }, 2000);
    } catch {
      showError(t('icsFeed.copyError'));
    }
  };

  return (
    <Card sx={{ mb: 2 }}>
      <CardContent sx={{ py: 1.5, '&:last-child': { pb: 1.5 } }}>
        <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
          <EventIcon sx={{ mr: 1, color: 'text.secondary', fontSize: 20 }} />
          <Typography variant="subtitle1" component="h2" sx={{ fontWeight: 500 }}>
            {t('icsFeed.title')}
          </Typography>
        </Box>
        <Divider sx={{ mb: 1.5 }} />
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
          {t('icsFeed.description')}
        </Typography>

        {loading && <CircularProgress size={24} />}
        {error && <Alert severity="error">{error}</Alert>}

        {!loading && !error && status && (
          <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
            {status.enabled ? (
              <>
                <Typography variant="body2" sx={{ mr: 1 }}>
                  {t('icsFeed.active', {
                    date: status.created_at ? new Date(status.created_at).toLocaleDateString() : '',
                  })}
                </Typography>
                <Typography variant="body2" color="text.secondary" sx={{ mr: 1 }}>
                  {status.last_fetched_at
                    ? t('icsFeed.lastFetched', { date: new Date(status.last_fetched_at).toLocaleString() })
                    : t('icsFeed.neverFetched')}
                </Typography>
                <Button
                  variant="outlined"
                  size="small"
                  disabled={busy}
                  onClick={() => {
                    setConfirm('regenerate');
                  }}
                >
                  {t('icsFeed.regenerateButton')}
                </Button>
                <Button
                  variant="outlined"
                  color="error"
                  size="small"
                  disabled={busy}
                  onClick={() => {
                    setConfirm('revoke');
                  }}
                >
                  {t('icsFeed.revokeButton')}
                </Button>
              </>
            ) : (
              <>
                <Typography variant="body2" sx={{ mr: 1 }}>
                  {t('icsFeed.inactive')}
                </Typography>
                <Button variant="outlined" size="small" disabled={busy} onClick={handleGenerate}>
                  {t('icsFeed.generateButton')}
                </Button>
              </>
            )}
          </Stack>
        )}
      </CardContent>

      <Dialog
        open={!!feedUrl}
        onClose={() => {
          setFeedUrl(null);
        }}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>{t('icsFeed.createdDialog.title')}</DialogTitle>
        <DialogContent>
          <Alert severity="warning" sx={{ mb: 2 }}>
            {t('icsFeed.createdDialog.warning')}
          </Alert>
          <Stack direction="row" alignItems="center" spacing={1}>
            <TextField
              label={t('icsFeed.createdDialog.urlLabel')}
              value={feedUrl || ''}
              InputProps={{ readOnly: true }}
              fullWidth
              size="small"
              inputProps={{ style: { fontFamily: 'monospace', fontSize: '0.85rem' } }}
            />
            <Tooltip title={copied ? t('icsFeed.createdDialog.copied') : t('icsFeed.createdDialog.copy')}>
              <IconButton
                onClick={handleCopy}
                color={copied ? 'success' : 'default'}
                aria-label={t('icsFeed.createdDialog.copy')}
              >
                <ContentCopyIcon />
              </IconButton>
            </Tooltip>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button
            variant="contained"
            onClick={() => {
              setFeedUrl(null);
            }}
          >
            {t('icsFeed.createdDialog.done')}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={confirm !== null}
        onClose={() => {
          setConfirm(null);
        }}
      >
        <DialogTitle>
          {confirm === 'revoke' ? t('icsFeed.revokeDialog.title') : t('icsFeed.regenerateDialog.title')}
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            {confirm === 'revoke' ? t('icsFeed.revokeDialog.message') : t('icsFeed.regenerateDialog.message')}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => {
              setConfirm(null);
            }}
            disabled={busy}
          >
            {t('common.cancel')}
          </Button>
          <Button
            variant="contained"
            color={confirm === 'revoke' ? 'error' : 'primary'}
            onClick={confirm === 'revoke' ? handleRevoke : handleGenerate}
            disabled={busy}
          >
            {confirm === 'revoke' ? t('icsFeed.revokeDialog.confirm') : t('icsFeed.regenerateDialog.confirm')}
          </Button>
        </DialogActions>
      </Dialog>
    </Card>
  );
}
