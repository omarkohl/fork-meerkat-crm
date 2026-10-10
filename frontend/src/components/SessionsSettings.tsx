import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Divider,
  Stack,
  Typography,
} from '@mui/material';
import DevicesIcon from '@mui/icons-material/Devices';
import { getSessions, revokeSession, signOutEverywhere, Session } from '../api/sessions';
import { logoutUser } from '../auth';
import { useSnackbar } from '../context/SnackbarContext';

export default function SessionsSettings() {
  const { t } = useTranslation();
  const { showSuccess, showError } = useSnackbar();
  const [sessions, setSessions] = useState<Session[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [busyId, setBusyId] = useState<string | null>(null);
  const [confirmAll, setConfirmAll] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      setSessions(await getSessions());
    } catch (err) {
      setError(err instanceof Error ? err.message : t('sessions.loadError'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const handleRevoke = async (session: Session) => {
    setBusyId(session.id);
    try {
      await revokeSession(session.id);
      showSuccess(t('sessions.revokeSuccess'));
      await load();
    } catch (err) {
      showError(err instanceof Error ? err.message : t('sessions.revokeError'));
    } finally {
      setBusyId(null);
    }
  };

  const handleSignOutEverywhere = async () => {
    setBusyId('all');
    try {
      await signOutEverywhere();
      // The server cleared the cookie; leave the app like a logout does.
      await logoutUser();
      window.location.href = '/login';
    } catch (err) {
      showError(err instanceof Error ? err.message : t('sessions.signOutAllError'));
      setBusyId(null);
      setConfirmAll(false);
    }
  };

  const formatDate = (value: string) => new Date(value).toLocaleString();

  return (
    <Card sx={{ mb: 2 }}>
      <CardContent sx={{ py: 1.5, '&:last-child': { pb: 1.5 } }}>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1 }}>
          <Box sx={{ display: 'flex', alignItems: 'center' }}>
            <DevicesIcon sx={{ mr: 1, color: 'text.secondary', fontSize: 20 }} />
            <Typography variant="subtitle1" component="h2" sx={{ fontWeight: 500 }}>
              {t('sessions.title')}
            </Typography>
          </Box>
          <Button
            size="small"
            color="error"
            variant="outlined"
            disabled={busyId !== null}
            onClick={() => setConfirmAll(true)}
          >
            {t('sessions.signOutAll')}
          </Button>
        </Box>
        <Divider sx={{ mb: 1.5 }} />
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
          {t('sessions.description')}
        </Typography>

        {loading && <CircularProgress size={24} />}
        {error && <Alert severity="error">{error}</Alert>}

        {!loading && !error && (
          <Stack component="ul" divider={<Divider component="li" />} sx={{ listStyle: 'none', p: 0, m: 0 }}>
            {sessions.map((session) => (
              <Box component="li" key={session.id} sx={{ display: 'flex', alignItems: 'center', py: 1, gap: 1 }}>
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Typography variant="body1" sx={{ fontWeight: 500 }} noWrap title={session.user_agent}>
                      {session.user_agent || t('sessions.unknownDevice')}
                    </Typography>
                    {session.current && <Chip size="small" color="primary" label={t('sessions.current')} />}
                  </Box>
                  <Typography variant="body2" color="text.secondary">
                    {session.ip && `${session.ip} · `}
                    {t('sessions.signedInAt', { date: formatDate(session.created_at) })} ·{' '}
                    {t('sessions.lastActive', { date: formatDate(session.last_seen_at) })}
                  </Typography>
                </Box>
                {!session.current && (
                  <Button
                    size="small"
                    color="error"
                    disabled={busyId !== null}
                    onClick={() => handleRevoke(session)}
                    aria-label={t('sessions.revokeAria', { device: session.user_agent || session.ip })}
                  >
                    {t('sessions.revoke')}
                  </Button>
                )}
              </Box>
            ))}
          </Stack>
        )}
      </CardContent>

      <Dialog open={confirmAll} onClose={() => setConfirmAll(false)}>
        <DialogTitle>{t('sessions.signOutAllDialog.title')}</DialogTitle>
        <DialogContent>
          <DialogContentText>{t('sessions.signOutAllDialog.message')}</DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmAll(false)} disabled={busyId !== null}>
            {t('common.cancel')}
          </Button>
          <Button color="error" variant="contained" onClick={handleSignOutEverywhere} disabled={busyId !== null}>
            {t('sessions.signOutAllDialog.confirm')}
          </Button>
        </DialogActions>
      </Dialog>
    </Card>
  );
}
