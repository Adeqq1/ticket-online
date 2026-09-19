import index from "./index.html";
import konser from "./konser.html";
import konserDetail from "./konser-detail.html";

const server = Bun.serve({
  routes: {
    "/": index,
    "/konser": konser,
    "/konser/:id": konserDetail,
  },
  development: {
    hmr: true,
    console: true,
  },
});

console.log(`Tiket Online tersedia di ${server.url}`);
