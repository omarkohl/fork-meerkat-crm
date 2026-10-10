import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Avatar,
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
  Tooltip,
  Typography,
} from '@mui/material';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline';
import DeleteForeverIcon from '@mui/icons-material/DeleteForever';
import RestoreFromTrashIcon from '@mui/icons-material/RestoreFromTrash';
import { useTrash } from './hooks/useTrash';
import { emptyTrash, purgeTrashedContact, restoreTrashedContact, TrashedContact } from './api/trash';
import { useSnackbar } from './context/SnackbarContext';
import { useDateFormat } from './DateFormatProvider';

function displayName(c: TrashedContact): string {
  return [c.firstname, c.nickname && `"${c.nickname}"`, c.lastname].filter(Boolean).join(' ');
}

export default function TrashPage() {
  const { t } = useTranslation();
  const { showSuccess, showError } = useSnackbar();
  const { formatDate } = useDateFormat();
  const { contacts, retentionDays, loading, error, refetch } = useTrash();

  const [busyId, setBusyId] = useState<number | null>(null);
  const [purgeTarget, setPurgeTarget] = useState<TrashedContact | null>(null);
  const [emptyOpen, setEmptyOpen] = useState(false);
  const [confirmBusy, setConfirmBusy] = useState(false);

  const handleRestore = async (contact: TrashedContact) => {
    setBusyId(contact.id);
    try {
      await restoreTrashedContact(contact.id);
      showSuccess(t('trash.restoreSuccess', { name: displayName(contact) }));
      await refetch();
    } catch (err) {
      showError(err instanceof Error ? err.message : t('trash.restoreError'));
    } finally {
      setBusyId(null);
    }
  };

  const handlePurge = async () => {
    if (!purgeTarget) return;
    setConfirmBusy(true);
    try {
      await purgeTrashedContact(purgeTarget.id);
      showSuccess(t('trash.purgeSuccess', { name: displayName(purgeTarget) }));
      setPurgeTarget(null);
      await refetch();
    } catch (err) {
      showError(err instanceof Error ? err.message : t('trash.purgeError'));
    } finally {
      setConfirmBusy(false);
    }
  };

  const handleEmpty = async () => {
    setConfirmBusy(true);
    try {
      const purged = await emptyTrash();
      showSuccess(t('trash.emptySuccess', { count: purged }));
      setEmptyOpen(false);
      await refetch();
    } catch (err) {
      showError(err instanceof Error ? err.message : t('trash.emptyError'));
    } finally {
      setConfirmBusy(false);
    }
  };

  return (
    <Box sx={{ maxWidth: 1200, mx: 'auto', mt: 2, p: 2 }}>
      <Typography variant="h5" component="h1" gutterBottom sx={{ mb: 1.5 }}>
        {t('trash.title')}
      </Typography>

      <Card sx={{ mb: 2 }}>
        <CardContent sx={{ py: 1.5, '&:last-child': { pb: 1.5 } }}>
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1 }}>
            <Box sx={{ display: 'flex', alignItems: 'center' }}>
              <DeleteOutlineIcon sx={{ mr: 1, color: 'text.secondary', fontSize: 20 }} />
              <Typography variant="subtitle1" component="h2" sx={{ fontWeight: 500 }}>
                {t('trash.contactsTitle')}
              </Typography>
            </Box>
            <Button
              variant="outlined"
              color="error"
              size="small"
              disabled={loading || contacts.length === 0}
              onClick={() => setEmptyOpen(true)}
            >
              {t('trash.emptyButton')}
            </Button>
          </Box>
          <Divider sx={{ mb: 1.5 }} />
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
            {t('trash.description', { count: retentionDays })}
          </Typography>

          {loading && <CircularProgress size={24} />}
          {error && <Alert severity="error">{error}</Alert>}
          {!loading && !error && contacts.length === 0 && (
            <Typography variant="body2" color="text.secondary">
              {t('trash.empty')}
            </Typography>
          )}

          {!loading && !error && contacts.length > 0 && (
            <Stack component="ul" divider={<Divider component="li" />} sx={{ listStyle: 'none', p: 0, m: 0 }}>
              {contacts.map((contact) => (
                <Box component="li" key={contact.id} sx={{ display: 'flex', alignItems: 'center', py: 1 }}>
                  <Avatar
                    src={contact.photo_thumbnail || undefined}
                    sx={{ width: 40, height: 40, mr: 1.5, bgcolor: 'primary.main' }}
                  >
                    {contact.firstname.charAt(0)}
                  </Avatar>
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Typography variant="body1" sx={{ fontWeight: 500 }} noWrap>
                      {displayName(contact)}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {t('trash.deletedOn', { date: formatDate(contact.deleted_at) })}
                      {' · '}
                      {t('trash.purgedOn', { date: formatDate(contact.purge_at) })}
                    </Typography>
                  </Box>
                  <Tooltip title={t('trash.restore')}>
                    <span>
                      <IconButton
                        aria-label={t('trash.restoreAria', { name: displayName(contact) })}
                        disabled={busyId === contact.id}
                        onClick={() => handleRestore(contact)}
                      >
                        <RestoreFromTrashIcon />
                      </IconButton>
                    </span>
                  </Tooltip>
                  <Tooltip title={t('trash.purge')}>
                    <span>
                      <IconButton
                        color="error"
                        aria-label={t('trash.purgeAria', { name: displayName(contact) })}
                        disabled={busyId === contact.id}
                        onClick={() => setPurgeTarget(contact)}
                      >
                        <DeleteForeverIcon />
                      </IconButton>
                    </span>
                  </Tooltip>
                </Box>
              ))}
            </Stack>
          )}
        </CardContent>
      </Card>

      <Dialog open={purgeTarget !== null} onClose={() => setPurgeTarget(null)}>
        <DialogTitle>{t('trash.purgeDialog.title')}</DialogTitle>
        <DialogContent>
          <DialogContentText>
            {t('trash.purgeDialog.message', { name: purgeTarget ? displayName(purgeTarget) : '' })}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPurgeTarget(null)} disabled={confirmBusy}>
            {t('common.cancel')}
          </Button>
          <Button color="error" variant="contained" onClick={handlePurge} disabled={confirmBusy}>
            {t('trash.purgeDialog.confirm')}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={emptyOpen} onClose={() => setEmptyOpen(false)}>
        <DialogTitle>{t('trash.emptyDialog.title')}</DialogTitle>
        <DialogContent>
          <DialogContentText>{t('trash.emptyDialog.message', { count: contacts.length })}</DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEmptyOpen(false)} disabled={confirmBusy}>
            {t('common.cancel')}
          </Button>
          <Button color="error" variant="contained" onClick={handleEmpty} disabled={confirmBusy}>
            {t('trash.emptyDialog.confirm')}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
