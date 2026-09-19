import index from "./index.html";
import konser from "./konser.html";
import konserDetail from "./konser-detail.html";
import checkout from "./checkout.html";

const server = Bun.serve({
  routes: {
    "/": index,
    "/konser": konser,
    "/konser/:id": konserDetail,
    "/checkout/:id": checkout,
  },
  development: {
    hmr: true,
    console: true,
  },
});

console.log(`Tiket Online tersedia di ${server.url}`);
