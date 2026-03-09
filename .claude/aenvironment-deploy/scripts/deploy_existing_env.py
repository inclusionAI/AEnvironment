#!/usr/bin/env python3
"""
Workflow C: Deploy Existing Environment

This workflow handles deployment of already registered environments:
1. Configure CLI
2. Verify environment exists
3. Deploy as instance or service
"""

import argparse
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
from aenv_operations import AEnvOperations, AEnvError


def main():
    parser = argparse.ArgumentParser(
        description="Deploy existing registered environment"
    )

    # Required parameters
    parser.add_argument("--env-spec", required=True,
                       help="Environment spec (name@version)")
    parser.add_argument("--owner-name", required=True,
                       help="Owner name")
    parser.add_argument("--api-service-url", required=True,
                       help="API service URL")
    parser.add_argument("--envhub-url", required=True,
                       help="EnvHub URL")

    # Deployment type
    parser.add_argument("--deploy-type", choices=["instance", "service"],
                       required=True, help="Deployment type")

    # Instance-specific options
    parser.add_argument("--ttl", default="24h",
                       help="Instance TTL (default: 24h)")
    parser.add_argument("--env-vars", type=json.loads, default={},
                       help='Environment variables as JSON')

    # Service-specific options
    parser.add_argument("--replicas", type=int, default=1,
                       help="Service replicas (default: 1)")
    parser.add_argument("--port", type=int, default=8080,
                       help="Service port (default: 8080)")
    parser.add_argument("--enable-storage", action="store_true",
                       help="Enable persistent storage for service")

    # Other options
    parser.add_argument("--verbose", action="store_true",
                       help="Verbose output")

    args = parser.parse_args()

    try:
        ops = AEnvOperations(verbose=args.verbose)

        print("Step 1/3: Configuring CLI...")
        ops.configure_cli(
            owner_name=args.owner_name,
            api_service_url=args.api_service_url,
            envhub_url=args.envhub_url,
            storage_type="local"
        )
        print("✓ CLI configured")

        print(f"\nStep 2/3: Verifying environment '{args.env_spec}'...")
        # Extract env name from spec (before @)
        env_name = args.env_spec.split('@')[0]

        try:
            result = ops.get_environment(env_name)
            print(f"✓ Environment exists")
            if args.verbose:
                print(f"\n{result['output']}")
        except AEnvError:
            print(f"\n⚠ Warning: Could not verify environment")
            print(f"  Continuing with deployment attempt...")

        print(f"\nStep 3/3: Deploying {args.deploy_type}...")
        if args.deploy_type == "instance":
            result = ops.create_instance(
                env_spec=args.env_spec,
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
                env_spec=args.env_spec,
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
