# Upstream CTFd Themes For heCsTackForse

This directory contains thin entrypoint wrappers for the original themes mirrored
under `/ctfd-themes`.

The original upstream repositories are copied into the project root at:

- `themes/`
- `themes/ctfd-theme-tsgctf-dist/` for the built TSGCTF distribution

Theme names exposed to the frontend loader:

- `CTFD-crimson-theme`
- `CTFD-odin-theme`
- `CTFd-Dark-Theme`
- `CTFd-xmas-theme`
- `UnitedStates`
- `ctfd-neon-theme`
- `ftheme`
- `nullify-ctfd-theme`
- `pixo`
- `r2-8bit-theme`
- `r2-terminal`
- `r2-witchcraft`
- `watchdogs`

`tsgctf` is mirrored in the repository, but it is a built Nuxt theme rather than a
plain CSS bundle, so it is not wired into the current CSS-only runtime loader.

