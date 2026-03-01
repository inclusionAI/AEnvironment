#!/usr/bin/env python3
"""
Workflow A: Deploy with Local Image Build

This workflow handles the complete process of:
1. Configure CLI
2. Initialize environment
3. Build Docker image locally
4. Register environment to EnvHub
5. Deploy as instance or service
"""

import argparse
import json
import sys
from pathlib import Path

# Add scripts directory to path
sys.path.insert(0, str(Path(__file__).parent))
from aenv_operations import AEnvOperations, AEnvError


def main():
    parser = argparse.ArgumentParser(
        description="Deploy with local image build"
    )

    # Required parameters
    parser.add_argument("--env-name", required=True,
                       help="Environment name")
    parser.add_argument("--owner-name", required=True,
                       help="Owner name")
    parser.add_argument("--api-service-url", required=True,
                       help="API service URL")
    parser.add_argument("--envhub-url", required=True,
                       help="EnvHub URL")

    # Registry configuration (required for local build)
    parser.add_argument("--registry-host", required=True,
                       help="Registry host")
    parser.add_argument("--registry-username", required=True,
                       help="Registry username")
    parser.add_argument("--registry-password", required=True,
                       help="Registry password")
    parser.add_argument("--registry-namespace", required=True,
                       help="Registry namespace")

    # Deployment type
    parser.add_argument("--deploy-type", choices=["instance", "service"],
                       required=True, help="Deployment type")

    # Instance-specific options
    parser.add_argument("--ttl", default="24h",
                       help="Instance TTL (default: 24h)")
    parser.add_argument("--env-vars", type=json.loads, default={},
                       help='Environment variables as JSON (e.g., \'{"KEY":"VALUE"}\')')

    # Service-specific options
    parser.add_argument("--replicas", type=int, default=1,
                       help="Service replicas (default: 1)")
    parser.add_argument("--port", type=int, default=8080,
                       help="Service port (default: 8080)")
    parser.add_argument("--enable-storage", action="store_true",
                       help="Enable persistent storage for service")

    # Other options
    parser.add_argument("--work-dir", default=".",
                       help="Working directory (default: current)")
    parser.add_argument("--platform", default="linux/amd64",
                       help="Build platform (default: linux/amd64)")
    parser.add_argument("--verbose", action="store_true",
                       help="Verbose output")

    args = parser.parse_args()

    try:
        ops = AEnvOperations(verbose=args.verbose)

        print("Step 1/5: Configuring CLI...")
        registry_config = {
            "host": args.registry_host,
            "username": args.registry_username,
            "password": args.registry_password,
            "namespace": args.registry_namespace
        }

        ops.configure_cli(
            owner_name=args.owner_name,
            api_service_url=args.api_service_url,
            envhub_url=args.envhub_url,
            storage_type="local",
            registry_config=registry_config
        )
        print("✓ CLI configured")

        print(f"\nStep 2/5: Initializing environment '{args.env_name}'...")
        result = ops.init_environment(
            env_name=args.env_name,
            config_only=False,
            target_dir=args.work_dir
        )
        env_dir = result["env_directory"]
        print(f"✓ Environment initialized at {env_dir}")

        print("\n⚠ IMPORTANT: Please edit the config.json file in the environment")
        print(f"   directory ({env_dir}) to customize:")
        print("   - CPU/memory requirements")
        print("   - Version number")
        print("   - Storage configuration (if needed)")
        print("\nPress Enter when ready to continue...")
        input()

        print(f"\nStep 3/5: Building Docker image...")
        ops.build_image(
            env_directory=env_dir,
            platform=args.platform,
            push=True
        )
        print("✓ Image built and pushed")

        print(f"\nStep 4/5: Registering environment to EnvHub...")
        result = ops.register_environment(env_dir)
        env_spec = f"{result['env_name']}@{result['version']}"
        print(f"✓ Environment registered: {env_spec}")

        print(f"\nStep 5/5: Deploying {args.deploy_type}...")
        if args.deploy_type == "instance":
            result = ops.create_instance(
                env_spec=env_spec,
                ttl=args.ttl,
                keep_alive=True,
                skip_health=True,
                env_vars=args.env_vars
            )
            print(f"✓ Instance created: {result['instance_id']}")
            print(f"  IP Address: {result['ip_address']}")
            if result.get('output'):
                print(f"\n{result['output']}")

        else:  # service
            result = ops.create_service(
                env_spec=env_spec,
                replicas=args.replicas,
                port=args.port,
                enable_storage=args.enable_storage,
                env_vars=args.env_vars
            )
            print(f"✓ Service created: {result['service_id']}")
            print(f"  Access URL: {result['access_url']}")
            if result.get('output'):
                print(f"\n{result['output']}")

        print("\n✅ Deployment completed successfully!")
        return 0

    except AEnvError as e:
        print(f"\n❌ Error: {e}", file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        print("\n\n⚠ Deployment cancelled by user", file=sys.stderr)
        return 130
    except Exception as e:
        print(f"\n❌ Unexpected error: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
