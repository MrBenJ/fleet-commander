import { ANTIGRAVITY_ICON_DATA_URI } from "./antigravityIconData";

export function AntigravityIcon({ size = 14 }: { size?: number }) {
  return (
    <img
      src={ANTIGRAVITY_ICON_DATA_URI}
      width={size}
      height={size}
      alt=""
      aria-hidden="true"
      style={{ display: "inline-block", verticalAlign: "middle" }}
    />
  );
}
