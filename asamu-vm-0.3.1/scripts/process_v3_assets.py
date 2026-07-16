from collections import deque
from hashlib import sha256
import json
import os
from pathlib import Path

from PIL import Image, ImageFilter, ImageStat


ROOT = Path(__file__).resolve().parents[1]
SOURCE = Path(os.environ.get("ASAMU_ASSET_SOURCE", ROOT / "asset-sources" / "v3"))
OUTPUT = ROOT / "apps" / "web" / "public" / "assets" / "default"


def connected_background_to_alpha(image: Image.Image) -> Image.Image:
    rgba = image.convert("RGBA")
    pixels = rgba.load()
    width, height = rgba.size
    queue: deque[int] = deque()
    visited = bytearray(width * height)

    def is_checkerboard(x: int, y: int) -> bool:
        red, green, blue, _ = pixels[x, y]
        return min(red, green, blue) > 218 and max(red, green, blue) - min(red, green, blue) < 16

    for x in range(width):
        queue.extend((x, (height - 1) * width + x))
    for y in range(height):
        queue.extend((y * width, y * width + width - 1))

    while queue:
        index = queue.popleft()
        if visited[index]:
            continue
        visited[index] = 1
        x, y = index % width, index // width
        if not is_checkerboard(x, y):
            continue
        red, green, blue, _ = pixels[x, y]
        pixels[x, y] = (red, green, blue, 0)
        if x > 0:
            queue.append(index - 1)
        if x + 1 < width:
            queue.append(index + 1)
        if y > 0:
            queue.append(index - width)
        if y + 1 < height:
            queue.append(index + width)

    alpha = rgba.getchannel("A").filter(ImageFilter.GaussianBlur(0.35))
    rgba.putalpha(alpha)
    return rgba


def dominant_color(image: Image.Image) -> str:
    sample = image.convert("RGB").resize((48, 48))
    red, green, blue = (round(value) for value in ImageStat.Stat(sample).median)
    return f"#{red:02x}{green:02x}{blue:02x}"


def write_asset(source: Path, relative: str, *, fit: str = "contain", position: str = "center", safe_area: dict | None = None) -> dict:
    destination = OUTPUT / relative
    destination.parent.mkdir(parents=True, exist_ok=True)
    metadata_path = destination.with_suffix(".metadata.json")
    thumb_path = destination.with_name(f"{destination.stem}.thumb.webp")
    if destination.exists() and thumb_path.exists() and metadata_path.exists():
        metadata = json.loads(metadata_path.read_text(encoding="utf-8"))
        return {"path": f"/assets/default/{relative}", "thumb": f"/assets/default/{thumb_path.relative_to(OUTPUT).as_posix()}", **metadata}

    cleaned = connected_background_to_alpha(Image.open(source))
    cleaned.save(destination, "WEBP", quality=92, method=6)

    thumb = cleaned.copy()
    thumb.thumbnail((256, 256), Image.Resampling.LANCZOS)
    thumb.save(thumb_path, "WEBP", quality=86, method=6)

    raw = destination.read_bytes()
    metadata = {
        "source": str(source.relative_to(SOURCE)).replace("\\", "/"),
        "width": cleaned.width,
        "height": cleaned.height,
        "aspectRatio": round(cleaned.width / cleaned.height, 4),
        "hasAlpha": True,
        "dominantColor": dominant_color(cleaned),
        "sha256": sha256(raw).hexdigest(),
        "safeArea": safe_area or {"top": 8, "right": 8, "bottom": 8, "left": 8},
        "focalPoint": {"x": 50, "y": 50},
        "recommendedObjectFit": fit,
        "recommendedObjectPosition": position,
        "version": 1,
    }
    metadata_path.write_text(json.dumps(metadata, ensure_ascii=False, indent=2), encoding="utf-8")
    return {"path": f"/assets/default/{relative}", "thumb": f"/assets/default/{thumb_path.relative_to(OUTPUT).as_posix()}", **metadata}


def numbered_files(folder: str) -> list[Path]:
    return sorted((SOURCE / folder).glob("*.png"), key=lambda path: path.name)


def main() -> None:
    manifest: dict[str, dict] = {}

    direct_assets = {
        "home.hero": (SOURCE / "首页 Hero 区.png", "home/home-hero.webp", "contain", "right center", {"top": 6, "right": 6, "bottom": 6, "left": 38}),
        "training.route.hero": (SOURCE / "训练路线推荐卡.png", "training/training-route-map.webp", "contain", "center", None),
        "competition.hero": (SOURCE / "比赛中心主视觉.png", "competitions/competition-hero.webp", "contain", "right center", {"top": 6, "right": 8, "bottom": 6, "left": 38}),
        "team.profile.banner": (SOURCE / "战队主页 Banner.png", "teams/team-profile-banner.webp", "contain", "center", None),
        "team.base.hero": (SOURCE / "战队基地推荐卡  战队广场主视觉.png", "teams/team-base-hero.webp", "contain", "right center", {"top": 8, "right": 8, "bottom": 8, "left": 42}),
        "team.announcement": (SOURCE / "战队公告.png", "teams/team-announcement.webp", "contain", "center", None),
    }
    for asset_key, (source, relative, fit, position, safe_area) in direct_assets.items():
        manifest[asset_key] = write_asset(source, relative, fit=fit, position=position, safe_area=safe_area)

    quick_keys = ["home.quick.start_training", "home.quick.join_competition", "home.quick.create_team", "home.quick.read_writeup"]
    quick_names = ["quick-start-training", "quick-join-competition", "quick-create-team", "quick-read-writeup"]
    for asset_key, name, source in zip(quick_keys, quick_names, numbered_files("首页四张素材")):
        manifest[asset_key] = write_asset(source, f"home/{name}.webp")

    direction_names = ["web", "ai_security", "pwn", "reverse", "crypto", "misc", "forensics", "iot", "mobile", "cloud"]
    direction_files = numbered_files("探索方向")
    # Lexicographic order places (10) after (1); map explicitly back to numeric order.
    direction_files = sorted(direction_files, key=lambda path: int(path.stem.split("(")[-1].rstrip(")")))
    numeric_direction_names = ["web", "pwn", "reverse", "crypto", "misc", "forensics", "iot", "mobile", "cloud", "ai_security"]
    for name, source in zip(numeric_direction_names, direction_files):
        manifest[f"direction.{name}.scene"] = write_asset(source, f"directions/{name.replace('_', '-')}.webp")

    character_names = ["student.male.default", "student.male.presenter", "student.female.default", "student.female.analyst"]
    for asset_key, source in zip(character_names, numbered_files("人物素材")):
        manifest[f"character.{asset_key}"] = write_asset(source, f"characters/{asset_key.replace('.', '-')}.webp")

    ranks = {
        "青铜": "bronze", "白银": "silver", "黄金": "gold", "铂金": "platinum",
        "钻石": "diamond", "大师": "master", "王者": "king", "超神": "legend",
    }
    for chinese, name in ranks.items():
        manifest[f"rank.{name}.main"] = write_asset(SOURCE / "等级素材" / f"{chinese}.png", f"ranks/{name}.webp")

    honor_keys = ["team.honor.champion", "team.honor.gold", "team.honor.silver", "team.honor.bronze", "team.honor.elite", "team.honor.verified"]
    honor_names = ["champion-trophy", "gold-medal", "silver-medal", "bronze-medal", "elite-three-star", "verified-team"]
    for asset_key, name, source in zip(honor_keys, honor_names, numbered_files("荣誉墙")):
        manifest[asset_key] = write_asset(source, f"honors/{name}.webp")

    # Existing environment status art remains the latest available source for this category.
    existing_environment = ROOT / "apps" / "web" / "public" / "assets" / "processed" / "environment"
    for status in ["idle", "starting", "running", "resetting", "failed", "expired", "maintenance"]:
        path = existing_environment / f"{status}.png"
        manifest[f"challenge.instance.{status}"] = {
            "path": f"/assets/processed/environment/{status}.png",
            "thumb": f"/assets/processed/environment/{status}.png",
            "width": Image.open(path).width,
            "height": Image.open(path).height,
            "aspectRatio": round(Image.open(path).width / Image.open(path).height, 4),
            "hasAlpha": True,
            "dominantColor": dominant_color(Image.open(path)),
            "sha256": sha256(path.read_bytes()).hexdigest(),
            "safeArea": {"top": 4, "right": 4, "bottom": 4, "left": 4},
            "focalPoint": {"x": 50, "y": 50},
            "recommendedObjectFit": "contain",
            "recommendedObjectPosition": "center",
            "version": 1,
        }

    manifest_path = OUTPUT / "manifest.v3.json"
    manifest_path.write_text(json.dumps({"schemaVersion": 3, "release": "2026.07.11-v3", "assets": manifest}, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"Generated {len(manifest)} semantic assets at {OUTPUT}")


if __name__ == "__main__":
    main()
