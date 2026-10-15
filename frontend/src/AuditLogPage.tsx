import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import {
  Alert,
  Box,
  CircularProgress,
  FormControl,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  Typography,
} from '@mui/material';
import { AuditEntry, getAuditLog } from './api/admin';
import { isAdmin } from './auth';
import { describeAuditDetail } from './utils/auditDetail';
import EmptyValue from './components/EmptyValue';

export default function AuditLogPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [events, setEvents] = useState<string[]>([]);
  const [total, setTotal] = useState(0);
  const [retentionDays, setRetentionDays] = useState(0);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(25);
  const [event, setEvent] = useState('');
  const [username, setUsername] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!isAdmin()) {
      navigate('/');
    }
  }, [navigate]);

  useEffect(() => {
    if (!isAdmin()) return;
    let cancelled = false;
    // Typing in the username filter waits a moment instead of asking on every key.
    const timer = setTimeout(async () => {
      setLoading(true);
      setError('');
      try {
        const result = await getAuditLog({ page: page + 1, limit: rowsPerPage, event, username: username.trim() });
        if (cancelled) return;
        setEntries(result.entries);
        setTotal(result.total);
        setEvents(result.events);
        setRetentionDays(result.retention_days);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : t('auditLog.loadError'));
      } finally {
        if (!cancelled) setLoading(false);
      }
    }, 250);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [page, rowsPerPage, event, username, t]);

  const eventLabel = (name: string) => t(`auditLog.events.${name}`, { defaultValue: name });

  return (
    <Box sx={{ maxWidth: 1200, mx: 'auto', mt: 2, p: 2 }}>
      <Typography variant="h5" component="h1" gutterBottom sx={{ mb: 1.5 }}>
        {t('auditLog.title')}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        {retentionDays > 0 ? t('auditLog.retention', { days: retentionDays }) : t('auditLog.retentionForever')}
      </Typography>

      <Box sx={{ display: 'flex', gap: 2, mb: 2, flexWrap: 'wrap' }}>
        <FormControl size="small" sx={{ minWidth: 220 }}>
          <InputLabel id="audit-event-label">{t('auditLog.filterEvent')}</InputLabel>
          <Select
            labelId="audit-event-label"
            label={t('auditLog.filterEvent')}
            value={event}
            onChange={(e) => {
              setEvent(e.target.value);
              setPage(0);
            }}
          >
            <MenuItem value="">{t('auditLog.allEvents')}</MenuItem>
            {events.map((name) => (
              <MenuItem key={name} value={name}>
                {eventLabel(name)}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <TextField
          size="small"
          label={t('auditLog.filterUser')}
          value={username}
          onChange={(e) => {
            setUsername(e.target.value);
            setPage(0);
          }}
        />
      </Box>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>{t('auditLog.time')}</TableCell>
              <TableCell>{t('auditLog.event')}</TableCell>
              <TableCell>{t('auditLog.user')}</TableCell>
              <TableCell>{t('auditLog.ip')}</TableCell>
              <TableCell>{t('auditLog.detail')}</TableCell>
              <TableCell>{t('auditLog.userAgent')}</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {loading && (
              <TableRow>
                <TableCell colSpan={6} align="center">
                  <CircularProgress size={24} />
                </TableCell>
              </TableRow>
            )}
            {!loading && entries.length === 0 && (
              <TableRow>
                <TableCell colSpan={6} align="center">
                  {t('auditLog.empty')}
                </TableCell>
              </TableRow>
            )}
            {!loading &&
              entries.map((entry) => (
                <TableRow key={entry.id}>
                  <TableCell sx={{ whiteSpace: 'nowrap' }}>{new Date(entry.created_at).toLocaleString()}</TableCell>
                  <TableCell>{eventLabel(entry.event)}</TableCell>
                  <TableCell>{entry.username || <EmptyValue />}</TableCell>
                  <TableCell>{entry.ip}</TableCell>
                  <TableCell>{describeAuditDetail(entry.detail, t)}</TableCell>
                  <TableCell sx={{ maxWidth: 260, wordBreak: 'break-word' }}>{entry.user_agent}</TableCell>
                </TableRow>
              ))}
          </TableBody>
        </Table>
        <TablePagination
          component="div"
          count={total}
          page={page}
          onPageChange={(_, p) => setPage(p)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={(e) => {
            setRowsPerPage(parseInt(e.target.value, 10));
            setPage(0);
          }}
          rowsPerPageOptions={[10, 25, 50, 100]}
        />
      </TableContainer>
    </Box>
  );
}
