{pkgs ? import <nixpkgs> {}}: let
  unstableTarball =
    fetchTarball
    https://github.com/NixOS/nixpkgs/archive/nixos-unstable.tar.gz;
  pkgs = import <nixpkgs> {};
  unstable = import unstableTarball {};
in
  pkgs.mkShell {
    nativeBuildInputs = with pkgs; [
      viddy # fancy watch
            redis
    ];
    shellHook = ''
    '';
  }
