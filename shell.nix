{pkgs ? import <nixpkgs> {}}: let
  unstableTarball =
    fetchTarball
    https://github.com/NixOS/nixpkgs/archive/nixos-unstable.tar.gz;
  pkgs = import <nixpkgs> {};
  unstable = import unstableTarball {};
in
  pkgs.mkShell {
    nativeBuildInputs = with pkgs; [
      unstable.lean4
      unstable.elan
      stdenv.cc.cc.lib # for python libstdc++6.so
      viddy # fancy watch
      redis
      python310Packages.huggingface-hub
    ];
    shellHook = ''
    '';
  }
