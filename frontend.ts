import { eventDate, featuredConcerts, type Concert } from "./concerts.ts";

const events = featuredConcerts;

export function filterEvents(query: string, source: Concert[] = events): Concert[] {
  const normalized = query.trim().toLocaleLowerCase("id-ID");
  if (!normalized) return source;
  return source.filter((event) =>
    [event.artist, event.city, event.venue].some((value) => value.toLocaleLowerCase("id-ID").includes(normalized)),
  );
}

if (typeof document !== "undefined") {
  const list = document.querySelector<HTMLDivElement>("#event-list");
  const form = document.querySelector<HTMLFormElement>("#event-search");
  const input = document.querySelector<HTMLInputElement>("#event-query");
  const emptyState = document.querySelector<HTMLDivElement>("#empty-state");
  const resetButton = document.querySelector<HTMLButtonElement>("#reset-search");

  function renderEvents(matches: Concert[]) {
    if (!list || !emptyState) return;
    list.replaceChildren(
      ...matches.map((event) => {
        const article = document.createElement("article");
        article.className = "event-card";
        const link = document.createElement("a");
        link.className = "event-card-link";
        link.href = `/konser/${event.id}`;
        link.innerHTML = `<img src="${event.image}" alt="Poster contoh konser ${event.artist}" width="900" height="1100" loading="lazy" /><div class="event-card-copy"><p>${eventDate(event.startsAt)}</p><h3>${event.artist}</h3><span>${event.venue}, ${event.city}</span></div>`;
        article.append(link);
        return article;
      }),
    );
    emptyState.hidden = matches.length > 0;
  }

  form?.addEventListener("submit", (event) => {
    event.preventDefault();
    renderEvents(filterEvents(input?.value ?? ""));
  });

  resetButton?.addEventListener("click", () => {
    if (input) input.value = "";
    renderEvents(events);
    input?.focus();
  });

  renderEvents(events);
}
