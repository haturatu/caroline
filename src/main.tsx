import { AppShell } from "./ui/app-shell.js";
// biome-ignore lint/correctness/noUnusedImports: TypeScript JSX factory import is a compiler input.
import { h } from "./ui/jsx-runtime.js";

const root = document.getElementById("app");
if (!root) throw new Error("Missing application mount point");

root.replaceChildren(<AppShell />);
void import("./app.js");
