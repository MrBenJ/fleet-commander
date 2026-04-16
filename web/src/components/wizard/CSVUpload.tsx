import { useState, useRef, useCallback } from "react";
import type { SquadronAgent } from "../../types";
import { parseAgentsCSV, type CSVParseError } from "../../utils/csvParser";

interface CSVUploadProps {
  squadronName: string;
  onAgentsParsed: (agents: SquadronAgent[]) => void;
}

export function CSVUpload({ squadronName, onAgentsParsed }: CSVUploadProps) {
  const [errors, setErrors] = useState<CSVParseError[]>([]);
  const [dragOver, setDragOver] = useState(false);
  const [lastImportCount, setLastImportCount] = useState<number | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const processFile = useCallback(
    (file: File) => {
      setErrors([]);
      setLastImportCount(null);

      if (!file.name.endsWith(".csv")) {
        setErrors([{ row: 0, message: "Please upload a .csv file" }]);
        return;
      }

      const reader = new FileReader();
      reader.onload = (e) => {
        const text = e.target?.result as string;
        const result = parseAgentsCSV(text, squadronName);
        if (result.errors.length > 0) {
          setErrors(result.errors);
        }
        if (result.agents.length > 0) {
          onAgentsParsed(result.agents);
          setLastImportCount(result.agents.length);
        }
      };
      reader.readAsText(file);
    },
    [squadronName, onAgentsParsed],
  );

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      setDragOver(false);
      const file = e.dataTransfer.files[0];
      if (file) processFile(file);
    },
    [processFile],
  );

  const handleFileChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (file) processFile(file);
      if (fileInputRef.current) fileInputRef.current.value = "";
    },
    [processFile],
  );

  return (
    <section
      aria-labelledby="csv-upload-heading"
      style={{
        border: `1px ${dragOver ? "solid" : "dashed"} ${dragOver ? "var(--blue)" : "var(--border)"}`,
        borderRadius: 8,
        padding: "1.5rem",
        background: dragOver
          ? "rgba(31,111,235,0.05)"
          : "var(--bg-secondary)",
        transition: "all 0.15s ease",
        marginBottom: "1.5rem",
      }}
      onDragOver={(e) => {
        e.preventDefault();
        setDragOver(true);
      }}
      onDragLeave={() => setDragOver(false)}
      onDrop={handleDrop}
    >
      <div
        id="csv-upload-heading"
        style={{
          fontWeight: 600,
          color: "var(--blue)",
          marginBottom: "0.5rem",
        }}
      >
        Import from CSV
      </div>
      <div style={{ display: "flex", alignItems: "center", gap: "0.5rem", marginBottom: "0.5rem" }}>
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ color: "var(--text-secondary)", flexShrink: 0 }}>
          <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
          <polyline points="7 10 12 15 17 10" />
          <line x1="12" y1="15" x2="12" y2="3" />
        </svg>
        <span style={{ fontWeight: 700, fontSize: "1rem", color: "var(--text-primary)" }}>
          Drag &amp; drop a CSV file here
        </span>
      </div>
      <p
        style={{
          color: "var(--text-secondary)",
          fontSize: "0.8rem",
          margin: "0 0 0.75rem",
        }}
      >
        Or click to browse. Columns:{" "}
        <code
          style={{
            background: "var(--bg-tertiary)",
            padding: "0.15rem 0.35rem",
            borderRadius: 3,
            fontSize: "0.75rem",
          }}
        >
          agent_name,branch,prompt,harness,persona
        </code>
      </p>

      <div style={{ display: "flex", gap: "0.75rem", alignItems: "center" }}>
        <input
          ref={fileInputRef}
          type="file"
          accept=".csv"
          onChange={handleFileChange}
          style={{ display: "none" }}
          aria-label="Upload CSV file"
        />
        <button
          type="button"
          onClick={() => fileInputRef.current?.click()}
          style={{
            background: "var(--bg-tertiary)",
            color: "var(--text-primary)",
            border: "1px solid var(--border)",
            borderRadius: 6,
            padding: "0.5rem 1rem",
            cursor: "pointer",
            fontSize: "0.85rem",
          }}
        >
          Choose CSV File
        </button>
        <a
          href="/sample-agents.csv"
          download="sample-agents.csv"
          style={{
            color: "var(--blue)",
            fontSize: "0.8rem",
            textDecoration: "none",
          }}
        >
          Download template
        </a>
      </div>

      {lastImportCount !== null && errors.length === 0 && (
        <div
          role="status"
          style={{
            marginTop: "0.75rem",
            color: "var(--green)",
            fontSize: "0.8rem",
          }}
        >
          Successfully imported {lastImportCount} agent
          {lastImportCount !== 1 ? "s" : ""}
        </div>
      )}

      {errors.length > 0 && (
        <div
          role="alert"
          style={{
            marginTop: "0.75rem",
            border: "1px solid rgba(248,81,73,0.3)",
            borderRadius: 6,
            padding: "0.75rem",
            background: "rgba(248,81,73,0.05)",
          }}
        >
          <div
            style={{
              fontWeight: 600,
              color: "var(--red, #f85149)",
              fontSize: "0.8rem",
              marginBottom: "0.35rem",
            }}
          >
            CSV Validation Errors
          </div>
          <ul
            style={{
              margin: 0,
              paddingLeft: "1.25rem",
              fontSize: "0.75rem",
              color: "var(--text-secondary)",
            }}
          >
            {errors.map((err, i) => (
              <li key={i}>
                {err.row > 0 && <>Row {err.row}: </>}
                {err.message}
              </li>
            ))}
          </ul>
          {lastImportCount !== null && lastImportCount > 0 && (
            <div
              style={{
                marginTop: "0.5rem",
                fontSize: "0.75rem",
                color: "var(--text-secondary)",
              }}
            >
              {lastImportCount} valid agent{lastImportCount !== 1 ? "s were" : " was"}{" "}
              still imported. Fix the errors above and re-upload to add the rest.
            </div>
          )}
        </div>
      )}
    </section>
  );
}
