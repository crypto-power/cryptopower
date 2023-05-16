# Bootstrappable Cryptopower Builds

This directory contains the files necessary to perform bootstrappable Cryptopower builds.

Bootstrappability furthers our binary security guarantees by allowing us to audit and reproduce our toolchain instead of blindly trusting binary downloads.

We achieve bootstrappability by using Guix as a functional package manager.

# Requirments

Conservatively, you will need an x86_64 machine with:

- 16GB of free disk space on the partition that /gnu/store will reside in
- 8GB of free disk space per platform triple you're planning on building (see the HOSTS environment variable description)

# Installation and Setup

If you don't have Guix installed and set up, please follow the instructions in INSTALL.md

# Usage



## Recognized environment variables

* _**HOSTS**_

  Override the space-separated list of platform triples for which to perform a
  bootstrappable build.

  _(defaults to "x86\_64-linux-gnu arm-linux-gnueabihf aarch64-linux-gnu
  riscv64-linux-gnu powerpc64-linux-gnu powerpc64le-linux-gnu
  x86\_64-w64-mingw32 x86\_64-apple-darwin arm64-apple-darwin")_

* _**SOURCES_PATH**_

  Set the depends tree download cache for sources. This is passed through to the
  depends tree. Setting this to the same directory across multiple builds of the
  depends tree can eliminate unnecessary redownloading of package sources.

  The path that this environment variable points to **must be a directory**, and
  **NOT a symlink to a directory**.

* _**BASE_CACHE**_

  Set the depends tree cache for built packages. This is passed through to the
  depends tree. Setting this to the same directory across multiple builds of the
  depends tree can eliminate unnecessary building of packages.

  The path that this environment variable points to **must be a directory**, and
  **NOT a symlink to a directory**.

* _**SDK_PATH**_

  Set the path where _extracted_ SDKs can be found. This is passed through to
  the depends tree. Note that this is should be set to the _parent_ directory of
  the actual SDK (e.g. `SDK_PATH=$HOME/Downloads/macOS-SDKs` instead of
  `$HOME/Downloads/macOS-SDKs/Xcode-12.2-12B45b-extracted-SDK-with-libcxx-headers`).

  The path that this environment variable points to **must be a directory**, and
  **NOT a symlink to a directory**.

* _**JOBS**_

  Override the number of jobs to run simultaneously, you might want to do so on
  a memory-limited machine. This may be passed to:

  - `guix` build commands as in `guix environment --cores="$JOBS"`
  - `make` as in `make --jobs="$JOBS"`
  - `xargs` as in `xargs -P"$JOBS"`

  See [here](#controlling-the-number-of-threads-used-by-guix-build-commands) for
  more details.

  _(defaults to the value of `nproc` outside the container)_

* _**SOURCE_DATE_EPOCH**_

  Override the reference UNIX timestamp used for bit-for-bit reproducibility,
  the variable name conforms to [standard][r12e/source-date-epoch].

  _(defaults to the output of `$(git log --format=%at -1)`)_

* _**V**_

  If non-empty, will pass `V=1` to all `make` invocations, making `make` output
  verbose.

  Note that any given value is ignored. The variable is only checked for
  emptiness. More concretely, this means that `V=` (setting `V` to the empty
  string) is interpreted the same way as not setting `V` at all, and that `V=0`
  has the same effect as `V=1`.

* _**SUBSTITUTE_URLS**_

  A whitespace-delimited list of URLs from which to download pre-built packages.
  A URL is only used if its signing key is authorized (refer to the [substitute
  servers section](#option-1-building-with-substitutes) for more details).

* _**ADDITIONAL_GUIX_COMMON_FLAGS**_

  Additional flags to be passed to all `guix` commands.

* _**ADDITIONAL_GUIX_TIMEMACHINE_FLAGS**_

  Additional flags to be passed to `guix time-machine`.

* _**ADDITIONAL_GUIX_ENVIRONMENT_FLAGS**_

  Additional flags to be passed to the invocation of `guix environment` inside
  `guix time-machine`.
