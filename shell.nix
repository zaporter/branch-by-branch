{pkgs ? import <nixpkgs> {}}: let
  unstableTarball =
    fetchTarball
    https://github.com/NixOS/nixpkgs/archive/nixos-unstable.tar.gz;
  pkgs = import <nixpkgs> {};
  unstable = import unstableTarball {};
in
  pkgs.mkShell {
    nativeBuildInputs = with pkgs; [
      gcc
      libgcc
      gnumake
      cmake
      lean
      elan
      # for lean
      unstable.stdenv.cc.cc.lib

      viddy # fancy watch
      redis
      python310Packages.huggingface-hub
      # backblaze
      rclone
    ];
    shellHook = ''
      # Nix can be so painful at times....
      # lean 4.15.0 requires CXXABI_1.3.15
      export LD_LIBRARY_PATH=${unstable.stdenv.cc.cc.lib}/lib:$LD_LIBRARY_PATH
    '';
  }
