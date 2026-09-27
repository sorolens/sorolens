import js from "@eslint/js";
import tseslint from "typescript-eslint";
import tailwind from "eslint-plugin-tailwindcss";

export default tseslint.config(
  js.configs.recommended,
  tseslint.configs.recommended,
  ...tailwind.configs["flat/recommended"]
);
