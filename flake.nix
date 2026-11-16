{
  description = "Go development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, utils }:
    utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        golangciConfig = builtins.fromJSON (builtins.readFile ./.golangci.json);
        golangci-lint = pkgs.buildGoModule {
          pname = "golangci-lint";
          version = golangciConfig.version;
          src = pkgs.fetchFromGitHub {
            owner = "golangci";
            repo = "golangci-lint";
            rev = "v${golangciConfig.version}";
            hash = "sha256-m9PoJHescRuT9DiKsb6LL+lQlc7RKAWDiRyf8pNMffA=";
          };
          vendorHash = golangciConfig.vendorHash;
          subPackages = [ "cmd/golangci-lint" ];
          ldflags = [
            "-s" "-w"
            "-X main.version=${golangciConfig.version}"
            "-X main.commit=nix"
            "-X main.date=nix"
          ];
        };
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.go
            pkgs.gopls
            pkgs.gotools
            golangci-lint
          ];
        };
      });
}
