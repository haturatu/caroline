import "./styles.css";
import { AppShell } from "./ui/app-shell";
// biome-ignore lint/correctness/noUnusedImports: TypeScript JSX factory import is a compiler input.
import { h } from "./ui/jsx-runtime";

const root = document.getElementById("app");
if (!root) throw new Error("Missing application mount point");

root.replaceChildren(<AppShell />);
void import("./app");
