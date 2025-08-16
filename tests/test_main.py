"""
Initial tests for the V-Architect application.
"""
import pytest
from v_architect import main

def test_main_entrypoint_exists():
    """
    Tests that the main function can be imported.
    """
    assert callable(main.main), "main.main should be a callable function"

def test_main_runs_without_error(capsys):
    """
    Tests that the main function runs without raising an exception and prints expected startup messages.
    """
    try:
        main.main()
        captured = capsys.readouterr()
        assert "Initializing V-Architect..." in captured.out
        assert "Welcome, Digital Architect." in captured.out
        assert "V-Architect initialization complete." in captured.out
    except Exception as e:
        pytest.fail(f"main.main() raised an exception unexpectedly: {e}")
