import index from "./index.html";
import konser from "./konser.html";

const server = Bun.serve({
  routes: {
    "/": index,
    "/konser": konser,
  },
  development: {
    hmr: true,
    console: true,
  },
});

console.log(`Tiket Online tersedia di ${server.url}`);
