import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
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
  Tooltip,
  Typography,
} from '@mui/material';
import BackupIcon from '@mui/icons-material/Backup';
import DeleteIcon from '@mui/icons-material/Delete';
import DownloadIcon from '@mui/icons-material/Download';
import { createBackup, deleteBackup, downloadBackup, getBackups, Backup, BackupList } from './api/backups';
import { isAdmin } from './auth';
import { useSnackbar } from './context/SnackbarContext';
import { handleFetchError } from './utils/errorHandler';

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export default function BackupsPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { showSuccess, showError } = useSnackbar();
  const [data, setData] = useState<BackupList | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [busyName, setBusyName] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Backup | null>(null);

  useEffect(() => {
    if (!isAdmin()) {
      navigate('/');
    }
  }, [navigate]);

  const refetch = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setData(await getBackups());
    } catch (err) {
      setError(handleFetchError(err, 'fetching backups'));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refetch();
  }, [refetch]);

  const handleCreate = async () => {
    setCreating(true);
    try {
      await createBackup();
      showSuccess(t('backups.createSuccess'));
      await refetch();
    } catch (err) {
      showError(err instanceof Error ? err.message : t('backups.createError'));
    } finally {
      setCreating(false);
    }
  };

  const handleDownload = async (backup: Backup) => {
    setBusyName(backup.name);
    try {
      await downloadBackup(backup.name);
    } catch (err) {
      showError(err instanceof Error ? err.message : t('backups.downloadError'));
    } finally {
      setBusyName(null);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setBusyName(deleteTarget.name);
    try {
      await deleteBackup(deleteTarget.name);
      showSuccess(t('backups.deleteSuccess'));
      setDeleteTarget(null);
      await refetch();
    } catch (err) {
      showError(err instanceof Error ? err.message : t('backups.deleteError'));
    } finally {
      setBusyName(null);
    }
  };

  const backups = data?.backups ?? [];

  return (
    <Box sx={{ maxWidth: 1200, mx: 'auto', mt: 2, p: 2 }}>
      <Typography variant="h5" component="h1" gutterBottom sx={{ mb: 1.5 }}>
        {t('backups.title')}
      </Typography>

      <Card sx={{ mb: 2 }}>
        <CardContent sx={{ py: 1.5, '&:last-child': { pb: 1.5 } }}>
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1 }}>
            <Box sx={{ display: 'flex', alignItems: 'center' }}>
              <BackupIcon sx={{ mr: 1, color: 'text.secondary', fontSize: 20 }} />
              <Typography variant="subtitle1" component="h2" sx={{ fontWeight: 500 }}>
                {t('backups.listTitle')}
              </Typography>
            </Box>
            <Button variant="contained" size="small" disabled={loading || creating} onClick={handleCreate}>
              {creating ? t('backups.creating') : t('backups.createButton')}
            </Button>
          </Box>
          <Divider sx={{ mb: 1.5 }} />

          <Alert severity="warning" sx={{ mb: 1.5 }}>
            {t('backups.warning')}
          </Alert>
          {data && (
            <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
              {data.enabled
                ? t('backups.scheduleOn', { time: data.time, count: data.retention_count })
                : t('backups.scheduleOff')}{' '}
              {t('backups.location', { directory: data.directory })}
            </Typography>
          )}

          {loading && <CircularProgress size={24} />}
          {error && <Alert severity="error">{error}</Alert>}
          {!loading && !error && backups.length === 0 && (
            <Typography variant="body2" color="text.secondary">
              {t('backups.empty')}
            </Typography>
          )}

          {!loading && !error && backups.length > 0 && (
            <Stack component="ul" divider={<Divider component="li" />} sx={{ listStyle: 'none', p: 0, m: 0 }}>
              {backups.map((backup) => (
                <Box component="li" key={backup.name} sx={{ display: 'flex', alignItems: 'center', py: 1 }}>
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Typography variant="body1" sx={{ fontWeight: 500 }} noWrap>
                      {new Date(backup.created_at).toLocaleString()}
                    </Typography>
                    <Typography variant="body2" color="text.secondary" noWrap>
                      {backup.name} · {formatSize(backup.size)}
                    </Typography>
                  </Box>
                  <Tooltip title={t('backups.download')}>
                    <span>
                      <IconButton
                        aria-label={t('backups.downloadAria', { name: backup.name })}
                        disabled={busyName === backup.name}
                        onClick={() => handleDownload(backup)}
                      >
                        <DownloadIcon />
                      </IconButton>
                    </span>
                  </Tooltip>
                  <Tooltip title={t('backups.delete')}>
                    <span>
                      <IconButton
                        color="error"
                        aria-label={t('backups.deleteAria', { name: backup.name })}
                        disabled={busyName === backup.name}
                        onClick={() => {
                          setDeleteTarget(backup);
                        }}
                      >
                        <DeleteIcon />
                      </IconButton>
                    </span>
                  </Tooltip>
                </Box>
              ))}
            </Stack>
          )}
        </CardContent>
      </Card>

      <Dialog
        open={deleteTarget !== null}
        onClose={() => {
          setDeleteTarget(null);
        }}
      >
        <DialogTitle>{t('backups.deleteDialog.title')}</DialogTitle>
        <DialogContent>
          <DialogContentText>{t('backups.deleteDialog.message', { name: deleteTarget?.name ?? '' })}</DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => {
              setDeleteTarget(null);
            }}
            disabled={busyName !== null}
          >
            {t('common.cancel')}
          </Button>
          <Button color="error" variant="contained" onClick={handleDelete} disabled={busyName !== null}>
            {t('backups.deleteDialog.confirm')}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
