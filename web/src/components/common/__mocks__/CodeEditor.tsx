/** Test mock for CodeEditor — renders a plain textarea for testing-library compatibility */
export function CodeEditor({
  value,
  onChange,
  labelId,
  placeholder,
}: {
  value: string;
  onChange: (val: string) => void;
  labelId?: string;
  placeholder?: string;
  minHeight?: number | string;
  language?: string;
}) {
  return (
    <textarea
      aria-labelledby={labelId}
      value={value}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
    />
  );
}
