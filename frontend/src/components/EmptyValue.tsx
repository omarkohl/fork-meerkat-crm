import { Typography, TypographyProps } from '@mui/material';

/** The placeholder shown wherever a value is not set, so empty fields look the same everywhere. */
export default function EmptyValue({ variant = 'inherit' }: { variant?: TypographyProps['variant'] }) {
  return (
    <Typography
      component="span"
      variant={variant}
      color="text.disabled"
      display={variant === 'inherit' ? 'inline' : 'block'}
    >
      —
    </Typography>
  );
}
