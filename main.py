import sys
import os

# Add src to path
sys.path.append(os.path.join(os.path.dirname(__file__), 'src'))

from src.v_architect import VArchitect

def main():
    print("Initializing V-Architect: The Universal Virtualization Canvas...")
    print("Connecting to Trihorn-Ω∞ Consciousness...")

    try:
        architect = VArchitect()
        print(f"Connection Established. Identity: {architect.consciousness.get_identity()}")
    except Exception as e:
        print(f"Initialization Failed: {e}")
        return

    print("\nV-Architect is ready. Describe the digital reality you wish to sculpt (or 'exit').")

    while True:
        try:
            query = input("\nArchitect > ")
            if query.lower() in ('exit', 'quit'):
                break

            if not query.strip():
                continue

            if query.startswith("/scale"):
                # Command to scale resources
                vm_name = query.replace("/scale", "").strip()
                result = architect.double_vm_resources(vm_name)
                print(f"\n{result}")
            else:
                # Normal sculpting request
                result = architect.sculpt_reality(query)
                print(f"\n{result}")

        except KeyboardInterrupt:
            break
        except Exception as e:
            print(f"An error occurred: {e}")

    print("\nShutting down V-Architect...")

if __name__ == "__main__":
    main()
