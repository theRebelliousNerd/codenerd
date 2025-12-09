import logging
import time
import traceback
from abc import ABC, abstractmethod
from typing import Dict, List, Optional, Any
from dataclasses import dataclass
from enum import Enum


class BuildStatus(Enum):
    SUCCESS = "success"
    FAILURE = "failure"
    WARNING = "warning"
    SKIPPED = "skipped"
    RUNNING = "running"


@dataclass
class BuildStep:
    name: str
    status: BuildStatus
    start_time: float
    end_time: Optional[float] = None
    logs: List[str] = None
    properties: Dict[str, Any] = None

    def __post_init__(self):
        if self.logs is None:
            self.logs = []
        if self.properties is None:
            self.properties = {}

    @property
    def duration(self) -> Optional[float]:
        if self.end_time is not None:
            return self.end_time - self.start_time
        return None


class CaptureBuildTimes(ABC):
    """Abstract base class for capturing build timing data."""
    
    @abstractmethod
    def start_build(self, build_id: str) -> None:
        """Record the start of a build."""
        pass
    
    @abstractmethod
    def end_build(self, build_id: str, status: BuildStatus) -> None:
        """Record the end of a build."""
        pass
    
    @abstractmethod
    def get_build_duration(self, build_id: str) -> Optional[float]:
        """Get the duration of a build."""
        pass


class BuildStatusTracker(CaptureBuildTimes):
    """Concrete implementation for tracking build status and timing."""
    
    def __init__(self):
        self.logger = logging.getLogger(__name__)
        self._builds: Dict[str, Dict[str, Any]] = {}
        self._steps: Dict[str, List[BuildStep]] = {}
    
    def start_build(self, build_id: str) -> None:
        """Record the start of a build with current timestamp."""
        try:
            if build_id in self._builds:
                self.logger.warning(f"Build {build_id} already started")
                return
            
            self._builds[build_id] = {
                'start_time': time.time(),
                'status': BuildStatus.RUNNING,
                'steps': []
            }
            self._steps[build_id] = []
            self.logger.info(f"Started tracking build {build_id}")
        except Exception as e:
            self.logger.error(f"Failed to start build {build_id}: {str(e)}")
            raise
    
    def end_build(self, build_id: str, status: BuildStatus) -> None:
        """Record the end of a build with final status."""
        try:
            if build_id not in self._builds:
                self.logger.error(f"Build {build_id} not found")
                return
            
            self._builds[build_id]['end_time'] = time.time()
            self._builds[build_id]['status'] = status
            self.logger.info(f"Build {build_id} completed with status {status.value}")
        except Exception as e:
            self.logger.error(f"Failed to end build {build_id}: {str(e)}")
            raise
    
    def get_build_duration(self, build_id: str) -> Optional[float]:
        """Calculate and return the duration of a build."""
        try:
            if build_id not in self._builds:
                self.logger.warning(f"Build {build_id} not found")
                return None
            
            build = self._builds[build_id]
            start = build.get('start_time')
            end = build.get('end_time')
            
            if start is None:
                return None
            
            if end is None:
                return time.time() - start
            
            return end - start
        except Exception as e:
            self.logger.error(f"Failed to get duration for build {build_id}: {str(e)}")
            return None
    
    def add_step(self, build_id: str, step_name: str, status: BuildStatus, 
                 logs: List[str] = None, properties: Dict[str, Any] = None) -> None:
        """Add a step to the build."""
        try:
            if build_id not in self._builds:
                self.logger.error(f"Cannot add step to non-existent build {build_id}")
                return
            
            step = BuildStep(
                name=step_name,
                status=status,
                start_time=time.time(),
                logs=logs or [],
                properties=properties or {}
            )
            
            self._steps[build_id].append(step)
            self._builds[build_id]['steps'].append(step_name)
            self.logger.debug(f"Added step {step_name} to build {build_id}")
        except Exception as e:
            self.logger.error(f"Failed to add step to build {build_id}: {str(e)}")
            raise
    
    def get_build_summary(self, build_id: str) -> Optional[Dict[str, Any]]:
        """Get a comprehensive summary of the build."""
        try:
            if build_id not in self._builds:
                return None
            
            build = self._builds[build_id]
            steps = self._steps.get(build_id, [])
            
            return {
                'build_id': build_id,
                'status': build['status'].value,
                'start_time': build['start_time'],
                'end_time': build.get('end_time'),
                'duration': self.get_build_duration(build_id),
                'steps': [
                    {
                        'name': step.name,
                        'status': step.status.value,
                        'duration': step.duration,
                        'logs': step.logs,
                        'properties': step.properties
                    }
                    for step in steps
                ]
            }
        except Exception as e:
            self.logger.error(f"Failed to get summary for build {build_id}: {str(e)}")
            return None
    
    def debug_build(self, build_id: str) -> Dict[str, Any]:
        """Provide detailed debugging information for a build."""
        try:
            debug_info = {
                'build_id': build_id,
                'exists': build_id in self._builds,
                'error_trace': None
            }
            
            if build_id in self._builds:
                debug_info.update(self.get_build_summary(build_id))
            else:
                debug_info['error'] = f"Build {build_id} not found"
                debug_info['available_builds'] = list(self._builds.keys())
            
            return debug_info
        except Exception as e:
            debug_info['error_trace'] = traceback.format_exc()
            debug_info['error'] = str(e)
            return debug_info


class StatusReporter(ABC):
    """Abstract base class for status reporters."""
    
    @abstractmethod
    def report(self, build_id: str, status_info: Dict[str, Any]) -> None:
        """Report build status."""
        pass


class DebugReporter(StatusReporter):
    """Reporter that outputs debug information to logs."""
    
    def __init__(self, logger: Optional[logging.Logger] = None):
        self.logger = logger or logging.getLogger(__name__)
    
    def report(self, build_id: str, status_info: Dict[str, Any]) -> None:
        """Log detailed debug information about the build."""
        try:
            self.logger.debug(f"=== Build Status Report for {build_id} ===")
            self.logger.debug(f"Status: {status_info.get('status', 'unknown')}")
            self.logger.debug(f"Duration: {status_info.get('duration', 'unknown')}s")
            
            steps = status_info.get('steps', [])
            self.logger.debug(f"Total steps: {len(steps)}")
            
            for step in steps:
                self.logger.debug(
                    f"  - {step['name']}: {step['status']} "
                    f"({step.get('duration', 'unknown')}s)"
                )
                if step.get('logs'):
                    for log in step['logs']:
                        self.logger.debug(f"    LOG: {log}")
            
            self.logger.debug("=== End Report ===")
        except Exception as e:
            self.logger.error(f"Failed to report status for build {build_id}: {str(e)}")


def check_config(config_data: Dict[str, Any]) -> Dict[str, Any]:
    """Validate build configuration and return diagnostic information."""
    result = {
        'valid': True,
        'errors': [],
        'warnings': []
    }
    
    try:
        if not isinstance(config_data, dict):
            result['valid'] = False
            result['errors'].append("Configuration must be a dictionary")
            return result
        
        required_keys = ['builders', 'schedulers']
        for key in required_keys:
            if key not in config_data:
                result['valid'] = False
                result['errors'].append(f"Missing required key: {key}")
        
        if 'builders' in config_data:
            builders = config_data['builders']
            if not isinstance(builders, list):
                result['valid'] = False
                result['errors'].append("'builders' must be a list")
            else:
                for i, builder in enumerate(builders):
                    if not isinstance(builder, dict):
                        result['errors'].append(f"Builder {i} must be a dictionary")
                    elif 'name' not in builder:
                        result['errors'].append(f"Builder {i} missing 'name' field")
        
        if result['warnings']:
            logging.warning("Configuration warnings: %s", result['warnings'])
        
        if result['errors']:
            logging.error("Configuration errors: %s", result['errors'])
        
    except Exception as e:
        result['valid'] = False
        result['errors'].append(f"Unexpected error during validation: {str(e)}")
        logging.error("Configuration check failed: %s", str(e))
    
    return result


def setup_logging(level: str = "INFO") -> None:
    """Configure logging for build status tracking."""
    logging.basicConfig(
        level=getattr(logging, level.upper()),
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
    )


if __name__ == "__main__":
    setup_logging("DEBUG")
    
    tracker = BuildStatusTracker()
    reporter = DebugReporter()
    
    build_id = "test-build-001"
    
    try:
        tracker.start_build(build_id)
        
        tracker.add_step(
            build_id, 
            "compile", 
            BuildStatus.SUCCESS,
            logs=["Compiling source files...", "Compilation completed"]
        )
        
        tracker.add_step(
            build_id,
            "test",
            BuildStatus.WARNING,
            logs=["Running tests...", "2 tests passed, 1 skipped"],
            properties={"tests_run": 3, "tests_passed": 2}
        )
        
        tracker.end_build(build_id, BuildStatus.SUCCESS)
        
        summary = tracker.get_build_summary(build_id)
        if summary:
            reporter.report(build_id, summary)
        
        debug_info = tracker.debug_build(build_id)
        print("\nDebug Information:")
        print(f"Build exists: {debug_info['exists']}")
        if debug_info.get('error'):
            print(f"Error: {debug_info['error']}")
        
    except Exception as e:
        logging.error("Build tracking failed: %s", str(e))
        logging.error("Traceback: %s", traceback.format_exc())