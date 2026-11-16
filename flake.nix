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
        toolVersions = builtins.fromJSON (builtins.readFile ./.tool-versions.json);
        golangci-lint = pkgs.buildGoModule {
          pname = "golangci-lint";
          version = toolVersions.golangci-lint.version;
          src = pkgs.fetchFromGitHub {
            owner = "golangci";
            repo = "golangci-lint";
            rev = "v${toolVersions.golangci-lint.version}";
            hash = "sha256-m9PoJHescRuT9DiKsb6LL+lQlc7RKAWDiRyf8pNMffA=";
          };
          vendorHash = toolVersions.golangci-lint.vendorHash;
          subPackages = [ "cmd/golangci-lint" ];
          ldflags = [
            "-s" "-w"
            "-X main.version=${toolVersions.golangci-lint.version}"
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
            pkgs.gnumake
            pkgs.jq
            golangci-lint
          ];
        };
      });
}
