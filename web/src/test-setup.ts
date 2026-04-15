import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";
import { createElement } from "react";

// jsdom doesn't implement scrollIntoView
Element.prototype.scrollIntoView = () => {};

// Mock @monaco-editor/react — Monaco doesn't work in jsdom.
// Render a div with the value so tests can assert on content.
vi.mock("@monaco-editor/react", () => ({
  default: (props: { value?: string; options?: { readOnly?: boolean } }) => {
    return createElement("div", {
      "data-testid": "monaco-editor",
      "data-readonly": props.options?.readOnly ? "true" : "false",
    }, props.value ?? "");
  },
}));

// Mock CodeEditor with a plain textarea for testing-library compatibility.
// The real component wraps Monaco Editor which doesn't render in jsdom.
vi.mock("./components/common/CodeEditor", () => ({
  CodeEditor: (props: { value: string; onChange: (val: string) => void; labelId?: string; placeholder?: string }) => {
    return createElement("textarea", {
      "aria-labelledby": props.labelId,
      value: props.value,
      placeholder: props.placeholder,
      onChange: (e: { target: { value: string } }) => props.onChange(e.target.value),
    });
  },
}));
