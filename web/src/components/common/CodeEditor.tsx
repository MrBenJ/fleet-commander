import { useEffect, useState } from "react";
import Editor from "@monaco-editor/react";

interface CodeEditorProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  minHeight?: number | string;
  language?: string;
  /** ID for the label element — used with aria-labelledby on the editor wrapper */
  labelId?: string;
}

function useTheme(): "vs-dark" | "light" {
  const [theme, setTheme] = useState<"vs-dark" | "light">(() => {
    const attr = document.documentElement.getAttribute("data-theme");
    return attr === "light" ? "light" : "vs-dark";
  });

  useEffect(() => {
    const observer = new MutationObserver(() => {
      const attr = document.documentElement.getAttribute("data-theme");
      setTheme(attr === "light" ? "light" : "vs-dark");
    });
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });
    return () => observer.disconnect();
  }, []);

  return theme;
}

export function CodeEditor({
  value,
  onChange,
  placeholder,
  minHeight = 200,
  language = "plaintext",
  labelId,
}: CodeEditorProps) {
  const monacoTheme = useTheme();
  const showPlaceholder = placeholder && !value;

  const heightValue =
    typeof minHeight === "number" ? `${minHeight}px` : minHeight;

  return (
    <div
      role="textbox"
      aria-labelledby={labelId}
      aria-multiline="true"
      style={{
        position: "relative",
        minHeight: heightValue,
        border: "1px solid var(--border)",
        borderRadius: 4,
        overflow: "hidden",
      }}
    >
      {showPlaceholder && (
        <div
          style={{
            position: "absolute",
            top: 8,
            left: 14,
            color: "var(--text-muted)",
            fontSize: 13,
            fontFamily: "'SF Mono', 'Fira Code', 'Fira Mono', monospace",
            pointerEvents: "none",
            zIndex: 1,
            whiteSpace: "pre-wrap",
            lineHeight: "18px",
          }}
        >
          {placeholder}
        </div>
      )}
      <Editor
        height={heightValue}
        defaultLanguage={language}
        value={value}
        theme={monacoTheme}
        onChange={(val) => onChange(val ?? "")}
        options={{
          minimap: { enabled: false },
          wordWrap: "on",
          lineNumbers: "off",
          scrollBeyondLastLine: false,
          renderLineHighlight: "none",
          overviewRulerLanes: 0,
          hideCursorInOverviewRuler: true,
          folding: false,
          glyphMargin: false,
          padding: { top: 8, bottom: 8 },
          fontSize: 13,
          scrollbar: {
            vertical: "auto",
            horizontal: "hidden",
            verticalScrollbarSize: 8,
          },
        }}
      />
    </div>
  );
}
