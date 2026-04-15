import { useEffect, useRef, useState, useCallback } from "react";
import Editor, { type OnMount } from "@monaco-editor/react";

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

/** Creates a style element for the gutter separator; returns it so callers can update it. */
function createGutterStyle(container: HTMLElement): HTMLStyleElement {
  const style = document.createElement("style");
  container.appendChild(style);
  return style;
}

/** Returns the CSS text for the gutter separator based on whether line numbers are shown. */
function gutterCSS(visible: boolean): string {
  if (!visible) return "";
  return `
    .monaco-editor .margin {
      border-right: 1px solid var(--border, rgba(128,128,128,0.35)) !important;
    }
    .monaco-editor .lines-content.monaco-editor-background {
      padding-left: 4px !important;
    }
  `;
}

export function CodeEditor({
  value,
  onChange,
  placeholder,
  minHeight = 200,
  language = "markdown",
  labelId,
}: CodeEditorProps) {
  const monacoTheme = useTheme();
  const [showLineNumbers, setShowLineNumbers] = useState(true);
  const gutterStyleRef = useRef<HTMLStyleElement | null>(null);
  const showPlaceholder = placeholder && !value;

  const heightValue =
    typeof minHeight === "number" ? `${minHeight}px` : minHeight;

  const showLineNumbersRef = useRef(showLineNumbers);
  showLineNumbersRef.current = showLineNumbers;

  const handleMount: OnMount = useCallback((editor) => {
    const container = editor.getDomNode();
    if (container) {
      const style = createGutterStyle(container);
      style.textContent = gutterCSS(showLineNumbersRef.current);
      gutterStyleRef.current = style;
    }
  }, []);

  useEffect(() => {
    if (gutterStyleRef.current) {
      gutterStyleRef.current.textContent = gutterCSS(showLineNumbers);
    }
  }, [showLineNumbers]);

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
      {/* Line-number toggle */}
      <button
        type="button"
        onClick={() => setShowLineNumbers((v) => !v)}
        title={showLineNumbers ? "Hide line numbers" : "Show line numbers"}
        style={{
          position: "absolute",
          top: 4,
          right: 4,
          zIndex: 10,
          background: "var(--bg-tertiary, #2d2d2d)",
          border: "1px solid var(--border, #555)",
          borderRadius: 4,
          color: "var(--text-secondary, #aaa)",
          fontSize: 11,
          padding: "2px 6px",
          cursor: "pointer",
          lineHeight: "16px",
          opacity: 0.8,
        }}
      >
        {showLineNumbers ? "#" : "¶"}
      </button>

      {showPlaceholder && (
        <div
          style={{
            position: "absolute",
            top: 8,
            left: showLineNumbers ? 50 : 14,
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
        onMount={handleMount}
        options={{
          minimap: { enabled: false },
          wordWrap: "on",
          lineNumbers: showLineNumbers ? "on" : "off",
          lineNumbersMinChars: 3,
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
