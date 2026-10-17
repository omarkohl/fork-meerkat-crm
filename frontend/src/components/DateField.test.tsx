import { describe, test, expect, vi, beforeEach } from 'vitest';
import { useState } from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import i18n from '../i18n/config';
import { DateFormat, DateFormatProvider } from '../DateFormatProvider';
import DateField, { parseDateInputWithFormat } from './DateField';

const renderIt = (format: DateFormat, initial = '', onChange = vi.fn()) => {
  window.localStorage.setItem('dateFormat', format);
  const Harness = () => {
    const [value, setValue] = useState(initial);
    return (
      <>
        <DateField
          label="From"
          value={value}
          onChange={(v) => {
            onChange(v);
            setValue(v);
          }}
        />
        <button
          onClick={() => {
            setValue('');
          }}
        >
          reset
        </button>
      </>
    );
  };
  render(
    <DateFormatProvider>
      <Harness />
    </DateFormatProvider>,
  );
  return { input: screen.getByRole('textbox', { name: 'From' }), onChange };
};

describe('parseDateInputWithFormat', () => {
  test('parses full dates in each format', () => {
    expect(parseDateInputWithFormat('30.04.2026', 'eu')).toBe('2026-04-30');
    expect(parseDateInputWithFormat('04/30/2026', 'us')).toBe('2026-04-30');
    expect(parseDateInputWithFormat('2026-04-30', 'iso')).toBe('2026-04-30');
    expect(parseDateInputWithFormat('', 'eu')).toBe('');
  });

  test('rejects year-less and impossible dates', () => {
    expect(parseDateInputWithFormat('30.04.', 'eu')).toBeNull();
    expect(parseDateInputWithFormat('31.02.2026', 'eu')).toBeNull();
    expect(parseDateInputWithFormat('30.04.20', 'eu')).toBeNull();
  });
});

describe('DateField', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('en');
  });

  test.each([
    ['eu', '30.04.2026'],
    ['us', '04/30/2026'],
    ['iso', '2026-04-30'],
  ] as const)('shows the value in the %s format', (format, shown) => {
    const { input } = renderIt(format, '2026-04-30');
    expect(input).toHaveValue(shown);
  });

  test('reports a typed date as YYYY-MM-DD', () => {
    const { input, onChange } = renderIt('eu');
    fireEvent.change(input, { target: { value: '30042026' } });
    expect(input).toHaveValue('30.04.2026');
    expect(onChange).toHaveBeenLastCalledWith('2026-04-30');
  });

  test('does not report incomplete input and marks it invalid on blur', () => {
    const { input, onChange } = renderIt('eu');
    fireEvent.change(input, { target: { value: '3004' } });
    expect(onChange).not.toHaveBeenCalled();
    fireEvent.blur(input);
    expect(screen.getByText('Invalid date')).toBeInTheDocument();
  });

  test('reports an incomplete edit of a valid date as empty, like a native date input', () => {
    const { input, onChange } = renderIt('eu', '2026-04-30');
    fireEvent.change(input, { target: { value: '30.04.202' } });
    expect(onChange).toHaveBeenLastCalledWith('');
    expect(input).toHaveValue('30.04.202');
  });

  test('clearing the text clears the value', () => {
    const { input, onChange } = renderIt('eu', '2026-04-30');
    fireEvent.change(input, { target: { value: '' } });
    expect(onChange).toHaveBeenLastCalledWith('');
  });

  test('follows a value cleared from outside', () => {
    const { input } = renderIt('eu', '2026-04-30');
    fireEvent.click(screen.getByRole('button', { name: 'reset' }));
    expect(input).toHaveValue('');
  });
});
