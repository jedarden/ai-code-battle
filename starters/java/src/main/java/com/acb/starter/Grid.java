package com.acb.starter;

import java.util.*;

/**
 * Grid utility functions for AI Code Battle.
 *
 * Provides toroidal distance calculations, neighbor enumeration,
 * and BFS pathfinding on a wrapping grid.
 */
public final class Grid {

    private static final int[][] OFFSETS = {
        {-1, -1}, {-1, 0}, {-1, 1},
        {0, -1},           {0, 1},
        {1, -1},  {1, 0},  {1, 1},
    };

    private Grid() {}

    /** Manhattan distance with wrap-around on a toroidal grid. */
    public static int toroidalManhattan(int r1, int c1, int r2, int c2, int rows, int cols) {
        int dr = Math.abs(r1 - r2);
        int dc = Math.abs(c1 - c2);
        dr = Math.min(dr, rows - dr);
        dc = Math.min(dc, cols - dc);
        return dr + dc;
    }

    /** Chebyshev distance with wrap-around on a toroidal grid. */
    public static int toroidalChebyshev(int r1, int c1, int r2, int c2, int rows, int cols) {
        int dr = Math.abs(r1 - r2);
        int dc = Math.abs(c1 - c2);
        dr = Math.min(dr, rows - dr);
        dc = Math.min(dc, cols - dc);
        return Math.max(dr, dc);
    }

    /** 8-directional neighbors with wrap-around. Returns [row, col] pairs. */
    public static List<int[]> neighbors(int row, int col, int rows, int cols) {
        List<int[]> result = new ArrayList<>(8);
        for (int[] off : OFFSETS) {
            result.add(new int[]{
                Math.floorMod(row + off[0], rows),
                Math.floorMod(col + off[1], cols),
            });
        }
        return result;
    }

    /**
     * BFS pathfinding on a toroidal grid.
     *
     * @param start    [row, col]
     * @param goal     [row, col]
     * @param passable predicate returning true if a cell can be entered
     * @param rows     grid height
     * @param cols     grid width
     * @return path as list of [row, col] (excluding start), or null if unreachable
     */
    public static List<int[]> bfs(int[] start, int[] goal,
                                  java.util.function.Predicate<int[]> passable,
                                  int rows, int cols) {
        if (start[0] == goal[0] && start[1] == goal[1]) {
            return Collections.emptyList();
        }

        Set<String> visited = new HashSet<>();
        visited.add(start[0] + "," + start[1]);

        Queue<int[]> posQueue = new ArrayDeque<>();
        Queue<List<int[]>> pathQueue = new ArrayDeque<>();
        posQueue.add(start);
        pathQueue.add(Collections.emptyList());

        while (!posQueue.isEmpty()) {
            int[] cur = posQueue.poll();
            List<int[]> path = pathQueue.poll();

            for (int[] nb : neighbors(cur[0], cur[1], rows, cols)) {
                List<int[]> newPath = new ArrayList<>(path);
                newPath.add(nb);

                if (nb[0] == goal[0] && nb[1] == goal[1]) {
                    return newPath;
                }

                String key = nb[0] + "," + nb[1];
                if (!visited.contains(key) && passable.test(nb)) {
                    visited.add(key);
                    posQueue.add(nb);
                    pathQueue.add(newPath);
                }
            }
        }
        return null;
    }
}
