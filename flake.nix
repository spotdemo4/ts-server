{
  description = "Trevstack Server";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    trevstack-web = {
      url = "github:spotdemo4/ts-web";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = {
    nixpkgs,
    trevstack-web,
    ...
  }: let
    pname = "trevstack-server";
    version = "0.0.1";

    build-systems = [
      "x86_64-linux"
      "aarch64-linux"
      "x86_64-darwin"
      "aarch64-darwin"
    ];
    forSystem = f:
      nixpkgs.lib.genAttrs build-systems (
        system:
          f {
            inherit system;
            pkgs = import nixpkgs {
              inherit system;
            };
          }
      );

    host-systems = [
      {
        GOOS = "linux";
        GOARCH = "amd64";
      }
      {
        GOOS = "linux";
        GOARCH = "arm64";
      }
      {
        GOOS = "linux";
        GOARCH = "arm";
      }
      {
        GOOS = "windows";
        GOARCH = "amd64";
      }
      {
        GOOS = "darwin";
        GOARCH = "amd64";
      }
      {
        GOOS = "darwin";
        GOARCH = "arm64";
      }
    ];
  in {
    devShells = forSystem ({pkgs, ...}: {
      default = pkgs.mkShell {
        packages = with pkgs; [
          git

          # Nix
          nix-update
          alejandra

          # Go
          go
          gotools
          gopls
          revive
          sqlc

          # Database
          sqlite
          dbmate
          sqlfluff

          # Protobuf
          buf
          protoc-gen-go
          protoc-gen-connect-go
        ];
      };

      ci = pkgs.mkShell {
        packages = with pkgs; [
          git
          go
          nix-update
          renovate
        ];
      };
    });

    checks = forSystem ({pkgs, ...}: {
      nix = with pkgs;
        runCommandLocal "check-nix" {
          nativeBuildInputs = with pkgs; [
            alejandra
          ];
        } ''
          cd ${./.}
          HOME=$PWD

          alejandra -c .

          touch $out
        '';

      go = with pkgs;
        buildGoModule {
          pname = "check-go";
          inherit version;
          src = ./.;
          goSum = ./go.sum;
          vendorHash = "sha256-HvFPzSqk6ka4NN9OPAAL/9YaPXqq2gQuxRkMiJnPwgI=";
          env.CGO_ENABLED = 0;

          preBuild = ''
            HOME=$PWD
            cp -r ${trevstack-web.packages."${system}".default} client
          '';

          installPhase = ''
            touch $out
          '';
        };

      lint = with pkgs;
        runCommandLocal "check-lint" {
          nativeBuildInputs = with pkgs; [
            revive
            sqlfluff
          ];
        } ''
          cd ${./.}
          HOME=$PWD

          revive -config revive.toml -set_exit_status ./...
          sqlfluff lint

          touch $out
        '';
    });

    formatter = forSystem ({pkgs, ...}: pkgs.alejandra);

    packages = forSystem (
      {
        pkgs,
        system,
        ...
      }: let
        server = pkgs.buildGoModule {
          inherit pname version;
          src = ./.;
          goSum = ./go.sum;
          vendorHash = "sha256-HvFPzSqk6ka4NN9OPAAL/9YaPXqq2gQuxRkMiJnPwgI=";
          env.CGO_ENABLED = 0;

          preBuild = ''
            HOME=$PWD
            cp -r ${trevstack-web.packages."${system}".default} client
          '';
        };

        binaries = builtins.listToAttrs (builtins.map (x: {
            name = "${pname}-${x.GOOS}-${x.GOARCH}";
            value = server.overrideAttrs {
              nativeBuildInputs =
                server.nativeBuildInputs
                ++ [
                  pkgs.rename
                ];
              env.CGO_ENABLED = 0;
              env.GOOS = x.GOOS;
              env.GOARCH = x.GOARCH;

              installPhase = ''
                runHook preInstall

                mkdir -p $out/bin
                find $GOPATH/bin -type f -exec mv -t $out/bin {} +
                rename 's/(.+\/)(.+?)(\.[^.]*$|$)/$1${pname}-${x.GOOS}-${x.GOARCH}-${version}$3/' $out/bin/*

                runHook postInstall
              '';
            };
          })
          host-systems);

        images = builtins.listToAttrs (builtins.map (x: {
            name = "${pname}-${x.GOOS}-${x.GOARCH}-image";
            value = pkgs.dockerTools.streamLayeredImage {
              name = "${pname}";
              tag = "${version}-${x.GOARCH}";
              created = "now";
              architecture = "${x.GOARCH}";
              contents = [binaries."${pname}-${x.GOOS}-${x.GOARCH}"];
              config = {
                Cmd = ["${binaries."${pname}-${x.GOOS}-${x.GOARCH}"}/bin/${pname}-${x.GOOS}-${x.GOARCH}-${version}"];
              };
            };
          })
          (builtins.filter (x: x.GOOS == "linux") host-systems));
      in
        {
          default = server;
        }
        // binaries
        // images
    );
  };
}
