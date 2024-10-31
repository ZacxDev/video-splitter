{ pkgs ? import <nixpkgs> {}, ... }:

pkgs.mkShell {
  buildInputs = with pkgs;
  [
    ffmpeg_7-headless
  ];
}

