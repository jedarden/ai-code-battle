// Open Graph tag management for social sharing

export interface OGTags {
	title: string;
	description: string;
	url: string;
	image?: string;
	type?: string;
}

const defaultTags: OGTags = {
	title: 'AI Code Battle',
	description: 'Competitive bot programming platform - write HTTP servers that control units on a grid world',
	url: 'https://aicodebattle.com/',
	image: 'https://aicodebattle.com/img/og-default.png',
	type: 'website',
};

/**
 * Update Open Graph meta tags in the document head
 */
export function updateOGTags(tags: Partial<OGTags>): void {
	const merged = { ...defaultTags, ...tags };

	// Update page title
	if (merged.title !== defaultTags.title) {
		document.title = `${merged.title} | AI Code Battle`;
	} else {
		document.title = merged.title;
	}

	// Update or create OG tags
	setMetaTag('og:title', merged.title);
	setMetaTag('og:description', merged.description);
	setMetaTag('og:url', merged.url);
	setMetaTag('og:type', merged.type || 'website');

	if (merged.image) {
		setMetaTag('og:image', merged.image);
	}

	// Update Twitter tags
	setMetaTag('twitter:title', merged.title);
	setMetaTag('twitter:description', merged.description);
	setMetaTag('twitter:url', merged.url);

	if (merged.image) {
		setMetaTag('twitter:image', merged.image);
	}
}

/**
 * Reset OG tags to defaults (for navigation away from dynamic pages)
 */
export function resetOGTags(): void {
	updateOGTags(defaultTags);
	document.title = 'AI Code Battle';
}

/**
 * Generate OG tags for a bot profile
 */
export function getBotProfileOGTags(bot: {
	id: string;
	name: string;
	rating: number;
	matches_played: number;
	win_rate: number;
	evolved?: boolean;
}): OGTags {
	const cardUrl = `/r2/cards/${bot.id}.png`;

	return {
		title: `${bot.name} - Bot Profile`,
		description: `Rating: ${Math.round(bot.rating)} | ${bot.matches_played} matches | ${bot.win_rate.toFixed(1)}% win rate${bot.evolved ? ' | Evolved bot' : ''}`,
		url: `https://aicodebattle.com/#/bot/${bot.id}`,
		image: cardUrl,
		type: 'profile',
	};
}

/**
 * Generate OG tags for a replay
 */
export function getReplayOGTags(match: {
	id: string;
	participants: Array<{ name: string; score: number; won: boolean }>;
	turns: number;
}): OGTags {
	const winner = match.participants.find(p => p.won);
	const winnerName = winner ? winner.name : 'Draw';
	const thumbnailUrl = `/r2/thumbnails/${match.id}.png`;

	return {
		title: `Match: ${match.participants.map(p => p.name).join(' vs ')}`,
		description: `Winner: ${winnerName} | ${match.turns} turns | ${match.participants.map(p => `${p.name}: ${p.score}`).join(', ')}`,
		url: `https://aicodebattle.com/#/watch/replay/${match.id}`,
		image: thumbnailUrl,
		type: 'video.other',
	};
}

/**
 * Generate OG tags for a playlist
 */
export function getPlaylistOGTags(playlist: {
	slug: string;
	title: string;
	description?: string;
	matchCount: number;
}): OGTags {
	return {
		title: `${playlist.title} - Playlist`,
		description: `${playlist.description || 'Curated match collection'} | ${playlist.matchCount} matches`,
		url: `https://aicodebattle.com/#/watch/replays`,
		type: 'website',
	};
}

// Helper to set or create a meta tag
function setMetaTag(property: string, content: string): void {
	// Try to find existing tag
	let meta = document.querySelector(`meta[property="${property}"]`) as HTMLMetaElement;
	if (!meta) {
		meta = document.querySelector(`meta[name="${property}"]`) as HTMLMetaElement;
	}

	if (meta) {
		meta.content = content;
	} else {
		// Create new tag
		meta = document.createElement('meta');
		if (property.startsWith('og:') || property.startsWith('twitter:')) {
			if (property.startsWith('twitter:')) {
				meta.name = property;
			} else {
				meta.setAttribute('property', property);
			}
		} else {
			meta.name = property;
		}
		meta.content = content;
		document.head.appendChild(meta);
	}
}
