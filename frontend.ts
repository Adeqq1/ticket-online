import "./index.css";

type Event = {
  artist: string;
  city: string;
  venue: string;
  date: string;
  image: string;
};

const events: Event[] = [
  {
    artist: "Nusa Malam",
    city: "Jakarta",
    venue: "Ruang Selatan",
    date: "Sabtu, 24 Agustus",
    image: "https://picsum.photos/seed/nusa-malam-jakarta/900/1100",
  },
  {
    artist: "Ruang Senja",
    city: "Bandung",
    venue: "Gudang Bunyi",
    date: "Jumat, 30 Agustus",
    image: "https://picsum.photos/seed/ruang-senja-bandung/900/1100",
  },
  {
    artist: "Dini Hari",
    city: "Yogyakarta",
    venue: "Balai Irama",
    date: "Minggu, 8 September",
    image: "https://picsum.photos/seed/dini-hari-yogyakarta/900/1100",
  },
  {
    artist: "Laut Kaca",
    city: "Surabaya",
    venue: "Panggung Timur",
    date: "Sabtu, 14 September",
    image: "https://picsum.photos/seed/laut-kaca-surabaya/900/1100",
  },
];

export function filterEvents(query: string, source = events): Event[] {
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

  function renderEvents(matches: Event[]) {
    if (!list || !emptyState) return;
    list.replaceChildren(
      ...matches.map((event) => {
        const article = document.createElement("article");
        article.className = "event-card";
        article.innerHTML = `
          <img src="${event.image}" alt="Suasana konser untuk contoh acara ${event.artist}" width="900" height="1100" loading="lazy" />
          <div class="event-card-copy">
            <p>${event.date}</p>
            <h3>${event.artist}</h3>
            <span>${event.venue}, ${event.city}</span>
          </div>`;
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
