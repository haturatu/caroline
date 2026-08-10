import { defineConfig } from "vite";

export default defineConfig({
	root: "web",
	publicDir: "public",
	build: {
		outDir: "../static",
		emptyOutDir: true,
	},
});
