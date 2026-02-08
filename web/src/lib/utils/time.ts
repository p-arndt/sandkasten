// Time formatting utilities

export function formatRelativeTime(isoString: string): string {
	if (!isoString) return '-';
	
	const date = new Date(isoString);
	const now = new Date();
	const diffMs = now.getTime() - date.getTime();
	const diffSec = Math.floor(diffMs / 1000);
	const diffMin = Math.floor(diffSec / 60);
	const diffHour = Math.floor(diffMin / 60);
	const diffDay = Math.floor(diffHour / 24);

	if (diffSec < 60) {
		return `${diffSec}s ago`;
	} else if (diffMin < 60) {
		return `${diffMin}m ago`;
	} else if (diffHour < 24) {
		return `${diffHour}h ago`;
	} else {
		return `${diffDay}d ago`;
	}
}

export function formatTimestamp(isoString: string): string {
	if (!isoString) return '-';
	
	const date = new Date(isoString);
	return date.toLocaleString();
}

export function formatUptime(seconds: number): string {
	const hours = Math.floor(seconds / 3600);
	const mins = Math.floor((seconds % 3600) / 60);
	const secs = seconds % 60;
	return `${hours}h ${mins}m ${secs}s`;
}

export function formatDuration(ms: number): string {
	if (ms < 1000) {
		return `${ms}ms`;
	}
	const seconds = Math.floor(ms / 1000);
	if (seconds < 60) {
		return `${seconds}s`;
	}
	const minutes = Math.floor(seconds / 60);
	const remainingSecs = seconds % 60;
	return `${minutes}m ${remainingSecs}s`;
}
