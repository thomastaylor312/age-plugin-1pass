{
  description = "age-plugin-1pass — age identity plugin backed by 1Password";

  # Flake inputs
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  # Flake outputs that other flakes can use
  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:

    # An explicit system list rather than `eachDefaultSystem`: nixpkgs 26.11
    # dropped support for x86_64-darwin, so merely evaluating that system
    # throws. Apple Silicon is the only Darwin target we build for now.
    flake-utils.lib.eachSystem
      [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ]
      (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};

          # Package version stamped into `main.version` at build time.
          # Bump this manually alongside the release tag.
          version = "v0.1.0";

          # The 1Password SDK's desktop-app auth path requires cgo, so every
          # build we produce keeps cgo enabled.
          #
          # On Linux we build through pkgsStatic, which swaps the stdenv to a
          # musl-based cross set so the resulting binary is fully statically
          # linked — matching what release.yml does for Linux artifacts. On
          # Darwin a truly static build isn't supported (libSystem must be
          # dynamically linked), so we fall back to the regular package set.
          isLinux = pkgs.stdenv.hostPlatform.isLinux;
          buildPkgs = if isLinux then pkgs.pkgsStatic else pkgs;

          age-plugin-1pass = buildPkgs.buildGo127Module {
            pname = "age-plugin-1pass";
            inherit version;

            src = ./.;

            vendorHash = "sha256-9ZgM6QP3uXLsRcsEktHTyNf6Txxus9sXyt175ivWG+c=";

            env.CGO_ENABLED = "1";

            ldflags = [
              "-s"
              "-w"
              "-X main.version=${version}"
            ]
            ++ pkgs.lib.optionals isLinux [
              # Force static linking of the cgo bits via musl.
              "-linkmode=external"
              "-extldflags=-static"
            ];

            # buildGoModule runs `go test ./...` in the check phase by default,
            # which gives us our test coverage for free.
            doCheck = true;

            meta = with pkgs.lib; {
              description = "age identity plugin backed by 1Password";
              homepage = "https://github.com/thomastaylor312/age-plugin-1pass";
              license = licenses.asl20;
              mainProgram = "age-plugin-1pass";
              platforms = [
                "x86_64-linux"
                "aarch64-linux"
                "aarch64-darwin"
              ];
            };
          };
        in
        {
          packages = {
            default = age-plugin-1pass;
            age-plugin-1pass = age-plugin-1pass;
          };

          # `nix flake check` builds the package (which runs `go test ./...`
          # in the check phase) so the same derivation serves as the test job.
          checks = {
            age-plugin-1pass = age-plugin-1pass;
          };

          devShells.default = pkgs.mkShell {
            packages = with pkgs; [
              go_1_27
              curl
              git
              jq
              wget
              delve
              just
              golangci-lint
            ];
          };
        }
      );
}
