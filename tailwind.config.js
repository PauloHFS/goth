/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./**/*.templ",
    "./**/*.go",
    "./**/*.html",
    "internal/view/**/*.templ",
    "internal/view/**/*.go",
    "web/**/*.templ",
    "web/**/*.go",
    "web/**/*.html"
  ],
  theme: {
    extend: {},
  },
}
